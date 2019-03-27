package mesh

import (
	"container/list"
	"sync"
)

type blockCache interface {
	Get(id BlockID) *BlockHeader
	put(b *BlockHeader)
	Close()
}

type MapCache struct {
	blockCache
	mp        map[BlockID]BlockHeader
	mutex     sync.RWMutex
	itemQueue *list.List
	cacheSize int
}

func NewMapCache(size int) MapCache {
	return MapCache{mp: map[BlockID]BlockHeader{}, itemQueue: list.New(), cacheSize: size}
}

func (mc MapCache) put(b *BlockHeader) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	if mc.itemQueue.Len() >= mc.cacheSize {
		mc.evict()
	}
	mc.mp[b.Id] = *b
	mc.itemQueue.PushBack(b.Id)
}

func (mc MapCache) evict() {
	if elem := mc.itemQueue.Front(); elem != nil {
		mc.itemQueue.Remove(elem)
		delete(mc.mp, elem.Value.(BlockID))
	}

}

func (mc MapCache) Get(id BlockID) *BlockHeader {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()
	if b, found := mc.mp[id]; found {
		return &b
	}
	return nil
}
