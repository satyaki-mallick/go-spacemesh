package mesh

import (
	"container/list"
)

type BlockCache struct {
	blockCache
}

type blockCache interface {
	Get(id BlockID) (*Block, error)
	Close()
}

func NewBlockCache(db blockCache) BlockCache {
	return BlockCache{db}
}

func (mc BlockCache) ForBlockInView(view map[BlockID]struct{}, layer LayerID, foo func(block *Block), errHandler func(err error)) {
	stack := list.New()
	for b := range view {
		stack.PushFront(b)
	}
	set := make(map[BlockID]struct{})
	for b := stack.Front(); b != nil; b = stack.Front() {
		a := stack.Remove(stack.Front()).(BlockID)
		block, err := mc.Get(a)
		if err != nil {
			errHandler(err)
		}
		foo(block)
		//push children to bfs queue
		for _, id := range block.ViewEdges {
			bChild, err := mc.Get(id)
			if err != nil {
				errHandler(err)
			}
			if bChild.Layer() >= layer { //dont traverse too deep
				if _, found := set[bChild.ID()]; !found {
					set[bChild.ID()] = struct{}{}
					stack.PushBack(bChild.ID())
				}
			}
		}
	}
	return
}
