package builders

import (
	"github.com/navknight/mgvm/mgpusim"
)

type HybridBuilder struct {
	*CommonBuilder
	switchL2TLBStriping bool
	usePtCaching        bool
}

func MakeHybridGPUBuilder() HybridBuilder {
	cbp := CommonBuilder{}
	b := HybridBuilder{CommonBuilder: &cbp, switchL2TLBStriping: true, usePtCaching: true}
	b.SetDefaultCommonBuilderParams()
	return b
}

func (b HybridBuilder) Build(name string, id uint64) *mgpusim.GPU {

}
