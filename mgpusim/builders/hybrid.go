package builders

import (
	"fmt"

	"gitlab.com/akita/akita"
	"gitlab.com/akita/mem/cache"
	"gitlab.com/akita/mem/vm/tlb"
	"gitlab.com/akita/mgpusim"
	"gitlab.com/akita/mgpusim/remotetranslation"
)

type HybridGPUBuilder struct {
	*CommonBuilder
	switchL2TLBStriping bool
	usePtCaching        bool
}

func MakeHybridGPUBuilder() HybridGPUBuilder {
	cbp := CommonBuilder{}
	b := HybridGPUBuilder{CommonBuilder: &cbp, switchL2TLBStriping: true, usePtCaching: true}
	b.SetDefaultCommonBuilderParams()
	return b
}

func (b HybridGPUBuilder) Build(name string, id uint64) *mgpusim.GPU {
	b.createGPU(name, id)

	b.buildCP()
	b.cp.SwitchL2TLBStriping(b.switchL2TLBStriping)

	chipRdmaAddressTable := b.createChipRDMAAddrTable()
	rdmaResponsePorts := make([]akita.Port, b.numChiplet)
	remoteAddressTranslationTable := b.createRemoteAddrTransTable()
	rtuResponsePorts := make([]akita.Port, b.numChiplet)

	for i := 0; i < b.numChiplet; i++ {
		chipletName := fmt.Sprintf("%s.chiplet_%02d", b.gpuName, i)
		chiplet := NewChiplet(chipletName, uint64(i))
		b.BuildSAs(chiplet)
		b.configChipRDMAEngine(chiplet, chipRdmaAddressTable, rdmaResponsePorts)

		if b.usePtCaching {
			b.buildMemBanks(chiplet)
		} else {
			b.CommonBuilder.buildMemBanks(chiplet)
		}

		b.buildMMU(chiplet)
		b.buildL2TLB(chiplet)

		b.configRemoteAddressTranslationUnit(chiplet, remoteAddressTranslationTable, rtuResponsePorts)

		b.connectL1ToL2(chiplet)
		if b.usePtCaching {
			b.connectL2ToDRAM(chiplet)
		} else {
			b.CommonBuilder.connectL2ToDRAM(chiplet)
		}

		b.connectL1TLBToL2TLB(chiplet)
		b.connectL2TLBTOMMU(chiplet)

		if b.usePtCaching {
			b.connectMMUToL2(chiplet)
		} else {
			b.CommonBuilder.connectMMUToL2(chiplet)
		}

		b.chiplets = append(b.chiplets, chiplet)
	}

	b.buildPageMigrationController()
	b.setupDMA()

	b.connectCP()
	b.setupInterchipNetwork()

	return b.gpu

}

// TODO: Complete the remote address translation table
func (b *HybridGPUBuilder) createRemoteAddrTransTable() *cache.HybridLowModuleFinder {
	remoteAddrTransTable := cache.NewHybridLowModuleFinder()
	return remoteAddrTransTable
}

func (b *HybridGPUBuilder) configRemoteAddressTranslationUnit(chiplet *Chiplet, remoteAddressTranslationTable *cache.HybridLowModuleFinder, rtuResponsePorts []akita.Port) {
	if b.useCoalescingRTU {
		chiplet.remoteTranslationUnit = remotetranslation.NewCoalescingRemoteTranslationUnit(
			fmt.Sprintf("%s.RTU", chiplet.name),
			b.engine,
			nil,
			nil,
		)
	} else {
		chiplet.remoteTranslationUnit = remotetranslation.NewRemoteTranslationUnit(
			fmt.Sprintf("%s.RTU", chiplet.name),
			b.engine,
			nil,
			nil,
		)
	}

	rtuResponsePorts[chiplet.ChipletID] = chiplet.remoteTranslationUnit.GetRequestPort()
	chiplet.remoteTranslationUnit.SetResponsePorts(rtuResponsePorts)
	chiplet.remoteTranslationUnit.(*remotetranslation.DefaultRTU).L2CtrlPort = chiplet.L2TLBs[0].GetControlPort()
	if b.useCoalescingRTU {
		chiplet.L2TLBs[0].(*tlb.LatTLB).ToRTU = chiplet.remoteTranslationUnit.(*remotetranslation.CoalescingRemoteTranslationUnit).ToL2
	} else {
		chiplet.L2TLBs[0].(*tlb.LatTLB).ToRTU = chiplet.remoteTranslationUnit.(*remotetranslation.DefaultRTU).ToL2
	}

	chiplet.remoteTranslationUnit.SetRemoteAddressTranslationTable(
		remoteAddressTranslationTable)
	remoteAddressTranslationTable.LowModules =
		append(remoteAddressTranslationTable.LowModules,
			chiplet.remoteTranslationUnit.GetRequestPort())
	b.gpu.RemoteAddressTranslationUnits =
		append(b.gpu.RemoteAddressTranslationUnits, chiplet.remoteTranslationUnit)
}
