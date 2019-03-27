package mesh

type blockCache interface {
	Get(id BlockID) *BlockHeader
	put(b *BlockHeader)
	Close()
}

type MapCache struct {
	blockCache
	mp map[BlockID]BlockHeader
}

func NewMapCache() MapCache {
	return MapCache{mp: map[BlockID]BlockHeader{}}
}

func (mc MapCache) put(b *BlockHeader) {
	mc.mp[b.Id] = *b
}

func (mc MapCache) Get(id BlockID) *BlockHeader {
	b, _ := mc.mp[id]
	return &b
}

func (mc MapCache) Close() {

}
