package activation

import (
	"github.com/spacemeshos/go-spacemesh/database"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/mesh"
	"github.com/spacemeshos/go-spacemesh/nipst"
	"github.com/spacemeshos/go-spacemesh/rand"
	"github.com/spacemeshos/go-spacemesh/types"
)

const AtxProtocol = "AtxGossip"

var activesetCache = NewActivesetCache(1000)

type ActiveSetProvider interface {
	GetActiveSetSize(l types.LayerID) uint32
}

type MeshProvider interface {
	GetLatestView() []types.BlockID
	LatestLayerId() types.LayerID
}

type EpochProvider interface {
	Epoch(l types.LayerID) types.EpochId
}

type Broadcaster interface {
	Broadcast(channel string, data []byte) error
}

type Builder struct {
	nodeId        types.NodeId
	db            *ActivationDb
	net           Broadcaster
	activeSet     ActiveSetProvider
	mesh          MeshProvider
	epochProvider EpochProvider
}

type Processor struct {
	db            *ActivationDb
	epochProvider EpochProvider
}

func NewBuilder(nodeId types.NodeId, db database.DB, meshdb *mesh.MeshDB, net Broadcaster, activeSet ActiveSetProvider, view MeshProvider, epochDuration EpochProvider, layersPerEpoch uint64) *Builder {
	return &Builder{
		nodeId, NewActivationDb(db, meshdb, layersPerEpoch), net, activeSet, view, epochDuration,
	}
}

func (b *Builder) PublishActivationTx(nipst *nipst.NIPST) error {
	var seq uint64
	prevAtx, err := b.GetPrevAtxId(b.nodeId)
	if prevAtx == nil {
		log.Info("previous ATX not found")
		prevAtx = &types.EmptyAtx
		seq = 0
	} else {
		seq = b.GetLastSequence(b.nodeId)
		if seq > 0 && prevAtx == nil {
			log.Error("cannot find prev ATX for nodeid %v ", b.nodeId)
			return err
		}
		seq++
	}

	l := b.mesh.LatestLayerId()
	ech := b.epochProvider.Epoch(l)
	var posAtx *types.AtxId = nil
	if ech > 0 {
		posAtx, err = b.GetPositioningAtxId(ech - 1)
		if err != nil {
			return err
		}
	} else {
		posAtx = &types.EmptyAtx
	}

	atx := types.NewActivationTx(b.nodeId, seq, *prevAtx, l, 0, *posAtx, b.activeSet.GetActiveSetSize(l-1), b.mesh.GetLatestView(), nipst)

	buf, err := types.AtxAsBytes(atx)
	if err != nil {
		return err
	}
	//todo: should we do something about it? wait for something?
	return b.net.Broadcast(AtxProtocol, buf)
}

func (b *Builder) GetPrevAtxId(node types.NodeId) (*types.AtxId, error) {
	ids, err := b.db.GetNodeAtxIds(node)

	if err != nil || len(ids) == 0 {
		return nil, err
	}
	return &ids[len(ids)-1], nil
}

func (b *Builder) GetPositioningAtxId(epochId types.EpochId) (*types.AtxId, error) {
	atxs, err := b.db.GetEpochAtxIds(epochId)
	if err != nil {
		return nil, err
	}
	atxId := atxs[rand.Int31n(int32(len(atxs)))]

	return &atxId, nil
}

func (b *Builder) GetLastSequence(node types.NodeId) uint64 {
	atxId, err := b.GetPrevAtxId(node)
	if err != nil {
		return 0
	}
	atx, err := b.db.GetAtx(*atxId)
	if err != nil {
		log.Error("wtf no atx in db %v", *atxId)
		return 0
	}
	return atx.Sequence
}

/*func (m *Mesh) UniqueAtxs(lyr *Layer) map[*ActivationTx]struct{} {
	atxMap := make(map[*ActivationTx]struct{})
	for _, blk := range lyr.blocks {
		for _, atx := range blk.ATXs {
			atxMap[atx] = struct{}{}
		}

	}
	return atxMap
}*/
