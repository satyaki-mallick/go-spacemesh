package activation

import (
	"github.com/spacemeshos/go-spacemesh/api"
	"github.com/spacemeshos/go-spacemesh/database"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/mesh"
	"github.com/spacemeshos/go-spacemesh/miner"
	"github.com/spacemeshos/go-spacemesh/nipst"
	"github.com/spacemeshos/go-spacemesh/rand"
)

type ActiveSetProvider interface {
	GetActiveSetSize(l mesh.LayerID) uint32
}

type MeshProvider interface {
	GetLatestView() []mesh.BlockID
	LatestLayerId() mesh.LayerID
}

type Builder struct {
	nodeId    mesh.NodeId
	db        *ActivationDb
	net       api.NetworkAPI
	activeSet ActiveSetProvider
	mesh      MeshProvider
}

type Processor struct {
	db *ActivationDb
}

func (p *Processor) ProcessBlockATXs(block *mesh.Block) {
	for _, atx := range block.ATXs {
		p.db.StoreAtx(atx)
	}
}

func NewBuilder(nodeId mesh.NodeId, db database.DB, net api.NetworkAPI, activeSet ActiveSetProvider, view MeshProvider) *Builder {
	return &Builder{
		nodeId, &ActivationDb{db}, net, activeSet, view,
	}
}

func (b Builder) BuildActivationTx(nipst *nipst.NIPST) error {
	prevAtx, err := b.GetPrevAtxId(b.nodeId)
	if err != nil {
		return err
	}
	l := b.mesh.LatestLayerId()
	posAtx, err := b.GetPositioningAtx(l - 1)
	if err != nil {
		return err
	}
	atx := mesh.NewActivationTx(b.nodeId, b.GetLastSequence(b.nodeId)+1, *prevAtx, l, 0, *posAtx, b.activeSet.GetActiveSetSize(l-1), b.mesh.GetLatestView(), nipst)

	buf, err := mesh.AtxAsBytes(atx)
	if err != nil {
		return err
	}
	//todo: should we do something about it? wait for something?
	return b.net.Broadcast(miner.AtxProtocol, buf)
}

func (b *Builder) GetPrevAtxId(node mesh.NodeId) (*mesh.AtxId, error) {
	ids, err := b.db.GetNodeAtxIds(node)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return &ids[len(ids)-1], nil
}

func (b *Builder) GetPositioningAtx(l mesh.LayerID) (*mesh.AtxId, error) {
	atxs, err := b.db.GetLayerAtxIds(l)
	if err != nil {
		return nil, err
	}
	if len(atxs) == 0 {
		//is this so?
		return nil, nil
	}
	atxId := atxs[rand.Int31n(int32(len(atxs)))]

	return &atxId, nil
}

func (b *Builder) GetLastSequence(node mesh.NodeId) uint64 {
	atxId, err := b.GetPrevAtxId(node)
	if err != nil {
		return 0
	}
	if atxId == nil {
		return 0
	}
	atx, err := b.db.GetAtx(*atxId)
	if err != nil {
		log.Error("wtf no atx in db %v", *atxId)
		return 0
	}
	return atx.Sequence
}
