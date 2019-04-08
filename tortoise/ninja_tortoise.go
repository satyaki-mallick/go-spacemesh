package tortoise

import (
	"fmt"
	"github.com/spacemeshos/go-spacemesh/common"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/mesh"
	"hash/fnv"
	"math"
	"sort"
	"sync"
)

type vec [2]int
type PatternId uint32

const ( //Threshold
	K               = 5 //number of explicit layers to vote for
	Window          = 100
	LocalThreshold  = 0.8 //ThetaL
	GlobalThreshold = 0.6 //ThetaG
	Genesis         = 0
)

var ( //correction vectors type
	//Opinion
	Support = vec{1, 0}
	Against = vec{0, 1}
	Abstain = vec{0, 0}
)

func Max(i mesh.LayerID, j mesh.LayerID) mesh.LayerID {
	if i > j {
		return i
	}
	return j
}

func (a vec) Add(v vec) vec {
	return vec{a[0] + v[0], a[1] + v[1]}
}

func (a vec) Negate() vec {
	a[0] = a[0] * -1
	a[1] = a[1] * -1
	return a
}

func (a vec) Multiply(x int) vec {
	a[0] = a[0] * x
	a[1] = a[1] * x
	return a
}

type votingPattern struct {
	id PatternId //cant put a slice here wont work well with maps, we need to hash the blockids
	mesh.LayerID
}

func (vp votingPattern) Layer() mesh.LayerID {
	return vp.LayerID
}

type BlockCache interface {
	GetBlock(id mesh.BlockID) (*mesh.Block, error)
	LayerBlockIds(id mesh.LayerID) ([]mesh.BlockID, error)
	ForBlockInView(view map[mesh.BlockID]struct{}, layer mesh.LayerID, foo func(block *mesh.BlockHeader), errHandler func(err error))
}

//todo memory optimizations
type ninjaTortoise struct {
	log.Log
	BlockCache         //block cache
	avgLayerSize       uint64
	pBase              votingPattern
	patterns           map[mesh.LayerID][]votingPattern                 //map patterns by layer for eviction purposes
	tEffective         map[mesh.BlockID]votingPattern                   //Explicit voting pattern of latest layer for a block
	tCorrect           map[mesh.BlockID]map[mesh.BlockID]vec            //correction vectors
	tExplicit          map[mesh.BlockID]map[mesh.LayerID]votingPattern  //explict votes from block to layer pattern
	tGood              map[mesh.LayerID]votingPattern                   //good pattern for layer i
	tSupport           map[votingPattern]int                            //for pattern p the number of blocks that support p
	tComplete          map[votingPattern]struct{}                       //complete voting patterns
	tEffectiveToBlocks map[votingPattern][]mesh.BlockID                 //inverse blocks effective pattern
	tVote              map[votingPattern]map[mesh.BlockID]vec           //global opinion
	tTally             map[votingPattern]map[mesh.BlockID]vec           //for pattern p and block b count votes for b according to p
	tPattern           map[votingPattern]map[mesh.BlockID]struct{}      //set of blocks that comprise pattern p
	tPatSupport        map[votingPattern]map[mesh.LayerID]votingPattern //pattern support count
}

func NewNinjaTortoise(layerSize int, blocks BlockCache, log log.Log) *ninjaTortoise {
	return &ninjaTortoise{
		Log:                log,
		BlockCache:         blocks,
		avgLayerSize:       uint64(layerSize),
		pBase:              votingPattern{},
		patterns:           map[mesh.LayerID][]votingPattern{},
		tGood:              map[mesh.LayerID]votingPattern{},
		tEffective:         map[mesh.BlockID]votingPattern{},
		tCorrect:           map[mesh.BlockID]map[mesh.BlockID]vec{},
		tExplicit:          map[mesh.BlockID]map[mesh.LayerID]votingPattern{},
		tSupport:           map[votingPattern]int{},
		tPattern:           map[votingPattern]map[mesh.BlockID]struct{}{},
		tVote:              map[votingPattern]map[mesh.BlockID]vec{},
		tTally:             map[votingPattern]map[mesh.BlockID]vec{},
		tComplete:          map[votingPattern]struct{}{},
		tEffectiveToBlocks: map[votingPattern][]mesh.BlockID{},
		tPatSupport:        map[votingPattern]map[mesh.LayerID]votingPattern{},
	}
}

func (ni *ninjaTortoise) evictOutOfWindow(idx mesh.LayerID) {
	wg := sync.WaitGroup{}
	if idx > Window {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, i := range ni.patterns[idx-Window-1] {
				delete(ni.tSupport, i)
				delete(ni.tComplete, i)
				delete(ni.tEffectiveToBlocks, i)
				delete(ni.tVote, i)
				delete(ni.tTally, i)
				delete(ni.tPattern, i)
				delete(ni.tPatSupport, i)
				delete(ni.tSupport, i)
				ni.Debug("evict pattern %v from maps ", i)
			}
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids, err := ni.LayerBlockIds(idx - Window - 1)
			if err != nil {
				ni.Error("could not get layer ids for layer ", idx-Window-1, err)
			}
			for _, i := range ids {
				delete(ni.tEffective, i)
				delete(ni.tCorrect, i)
				delete(ni.tExplicit, i)
				ni.Debug("evict block %v from maps ", i)
			}
		}()
	}
	wg.Wait()
}

func (ni *ninjaTortoise) processBlock(b *mesh.Block) {

	ni.Debug("process block: %d layer: %d  ", b.ID(), b.Layer())
	if b.Layer() == Genesis {
		return
	}

	patternMap := make(map[mesh.LayerID]map[mesh.BlockID]struct{})
	for _, bid := range b.BlockVotes {
		ni.Debug("block votes %d", bid)
		bl, err := ni.GetBlock(bid)
		if err != nil || bl == nil {
			ni.Error(fmt.Sprintf("error block not found ID %d !!!!!", bid))
			return
		}
		if _, found := patternMap[bl.Layer()]; !found {
			patternMap[bl.Layer()] = map[mesh.BlockID]struct{}{}
		}
		patternMap[bl.Layer()][bl.ID()] = struct{}{}
	}

	var effective votingPattern
	ni.tExplicit[b.ID()] = make(map[mesh.LayerID]votingPattern, K)
	for layerId, v := range patternMap {
		vp := votingPattern{id: getIdsFromSet(v), LayerID: layerId}
		ni.tPattern[vp] = v
		arr, _ := ni.patterns[vp.Layer()]
		ni.patterns[vp.Layer()] = append(arr, vp)
		ni.tExplicit[b.ID()][layerId] = vp
		if layerId >= effective.Layer() {
			effective = vp
		}
	}

	ni.tEffective[b.ID()] = effective

	v, found := ni.tEffectiveToBlocks[effective]
	if !found {
		v = make([]mesh.BlockID, 0, ni.avgLayerSize)
	}
	var pattern []mesh.BlockID = nil
	pattern = append(v, b.ID())
	ni.tEffectiveToBlocks[effective] = pattern
	ni.Debug("effective pattern to blocks %d %d", effective, pattern)

	return
}

func getId(bids []mesh.BlockID) PatternId {
	sort.Slice(bids, func(i, j int) bool { return bids[i] < bids[j] })
	// calc
	h := fnv.New32()
	for i := 0; i < len(bids); i++ {
		h.Write(common.Uint32ToBytes(uint32(bids[i])))
	}
	// update
	sum := h.Sum32()
	return PatternId(sum)
}

func getIdsFromSet(bids map[mesh.BlockID]struct{}) PatternId {
	keys := make([]mesh.BlockID, 0, len(bids))
	for k := range bids {
		keys = append(keys, k)
	}
	return getId(keys)
}

func globalOpinion(v vec, layerSize uint64, delta float64) vec {
	threshold := float64(GlobalThreshold*delta) * float64(layerSize)
	if float64(v[0]) > threshold {
		return Support
	} else if float64(v[1]) > threshold {
		return Against
	} else {
		return Abstain
	}
}

func (ni *ninjaTortoise) updateCorrectionVectors(p votingPattern, bottomOfWindow mesh.LayerID) {
	foo := func(x *mesh.BlockHeader) {
		for _, bid := range ni.tEffectiveToBlocks[p] { //for all b who's effective vote is p
			b, err := ni.GetBlock(bid)
			if err != nil {
				panic(fmt.Sprintf("error block not found ID %d", bid))
			}

			if _, found := ni.tExplicit[b.ID()][x.Layer()]; found { //if Texplicit[b][x.layer]!=0 check correctness of x.layer and found
				ni.Debug(" blocks pattern %d block %d layer %d", p, b.ID(), b.Layer())
				if _, found := ni.tCorrect[b.ID()]; !found {
					ni.tCorrect[b.ID()] = make(map[mesh.BlockID]vec)
				}
				vo := ni.tVote[p][x.ID()]
				ni.Debug("vote from pattern %d to block %d layer %d vote %d ", p, x.ID(), x.Layer(), vo)
				ni.tCorrect[b.ID()][x.ID()] = vo.Negate() //Tcorrect[b][x] = -Tvote[p][x]
				ni.Debug("update correction vector for block %d layer %d , pattern %d vote %d for block %d ", b.ID(), b.Layer(), p, ni.tCorrect[b.ID()][x.ID()], x.ID())
			} else {
				ni.Debug("block %d from layer %d dose'nt explicitly vote for layer %d", b.ID(), b.Layer(), x.Layer())
			}
		}
	}

	ni.ForBlockInView(ni.tPattern[p], bottomOfWindow, foo, func(err error) {})
}

func (ni *ninjaTortoise) updatePatternTally(newMinGood votingPattern, botomOfWindow mesh.LayerID, correctionMap map[mesh.BlockID]vec, effCountMap map[mesh.LayerID]int) {
	ni.Debug("update tally pbase id:%d layer:%d p id:%d layer:%d", ni.pBase.id, ni.pBase.Layer(), newMinGood.id, newMinGood.Layer())
	for idx, effc := range effCountMap {
		g := ni.tGood[idx]
		for b, v := range ni.tVote[g] {
			tally := ni.tTally[newMinGood][b]
			tally = tally.Add(v.Multiply(effc))
			if count, found := correctionMap[b]; found {
				tally = tally.Add(count)
			} else {
				ni.Debug("no correction vectors for %", g)
			}
			ni.Debug("tally for pattern %d  and block %d is %d", newMinGood.id, b, tally)
			ni.tTally[newMinGood][b] = tally //in g's view -> in p's view
		}
	}
}

func (ni *ninjaTortoise) getCorrEffCounter() (map[mesh.BlockID]vec, map[mesh.LayerID]int, func(b *mesh.BlockHeader)) {
	correctionMap := make(map[mesh.BlockID]vec)
	effCountMap := make(map[mesh.LayerID]int)
	foo := func(b *mesh.BlockHeader) {
		if b.Layer() > ni.pBase.Layer() { //because we already copied pbase's votes
			if eff, found := ni.tEffective[b.ID()]; found {
				if p, found := ni.tGood[eff.Layer()]; found && eff == p {
					effCountMap[eff.Layer()] = effCountMap[eff.Layer()] + 1
					for k, v := range ni.tCorrect[b.ID()] {
						correctionMap[k] = correctionMap[k].Add(v)
					}
				}
			}
		}
	}
	return correctionMap, effCountMap, foo
}

//for all layers from pBase to i add b's votes, mark good layers
// return new minimal good layer
func (ni *ninjaTortoise) findMinimalNewlyGoodLayer(lyr *mesh.Layer) mesh.LayerID {
	minGood := mesh.LayerID(math.MaxUint64)

	var j mesh.LayerID
	if Window > lyr.Index() {
		j = ni.pBase.Layer() + 1
	} else {
		j = Max(ni.pBase.Layer()+1, lyr.Index()-Window+1)
	}

	for ; j < lyr.Index(); j++ {
		// update block votes on all patterns in blocks view
		sUpdated := ni.updateBlocksSupport(lyr.Blocks(), j)
		//todo do this as part of previous for if possible
		//for each p that was updated and not the good layer of j check if it is the good layer
		for p := range sUpdated {
			//if a majority supports p (p is good)
			//according to tal we dont have to know the exact amount, we can multiply layer size by number of layers
			jGood, found := ni.tGood[j]
			threshold := 0.5 * float64(mesh.LayerID(ni.avgLayerSize)*(lyr.Index()-p.Layer()))

			if (jGood != p || !found) && float64(ni.tSupport[p]) > threshold {
				ni.tGood[p.Layer()] = p
				//if p is the new minimal good layer
				if p.Layer() < minGood {
					minGood = p.Layer()
				}
			}
		}
	}
	ni.Debug("found minimal good layer %d", minGood)
	return minGood
}

//update block support for pattern in layer j
func (ni *ninjaTortoise) updateBlocksSupport(b []*mesh.Block, j mesh.LayerID) map[votingPattern]struct{} {
	sUpdated := map[votingPattern]struct{}{}
	for _, block := range b {
		//check if block votes for layer j explicitly or implicitly
		p, found := ni.tExplicit[block.ID()][j]
		if found {
			//explicit
			ni.tSupport[p]++         //add to supporting patterns
			sUpdated[p] = struct{}{} //add to updated patterns

			//implicit
		} else if eff, effFound := ni.tEffective[block.ID()]; effFound {
			p, found = ni.tPatSupport[eff][j]
			if found {
				ni.tSupport[p]++         //add to supporting patterns
				sUpdated[p] = struct{}{} //add to updated patterns
			}
		}
	}
	return sUpdated
}

func (ni *ninjaTortoise) addPatternVote(p votingPattern, view map[mesh.BlockID]struct{}) func(b mesh.BlockID) {
	addPatternVote := func(b mesh.BlockID) {
		var vp map[mesh.LayerID]votingPattern
		var found bool
		bl, err := ni.GetBlock(b)
		if err != nil {
			panic(fmt.Sprintf("error block not found ID %d", b))
		}
		if bl.Layer() <= ni.pBase.Layer() {
			return
		}

		if vp, found = ni.tExplicit[b]; !found {
			panic(fmt.Sprintf("block %d has no explicit voting, something went wrong ", b))
		}
		for _, ex := range vp {
			blocks, err := ni.LayerBlockIds(ex.Layer()) //todo handle error
			if err != nil {
				panic("could not retrieve layer block ids")
			}
			for _, bl := range blocks {
				if _, found := ni.tPattern[ex][bl]; found {
					ni.tTally[p][bl] = ni.tTally[p][bl].Add(Support)
				} else if _, inSet := view[bl]; inSet { //in view but not in pattern
					ni.tTally[p][bl] = ni.tTally[p][bl].Add(Against)
				}
			}
		}
	}
	return addPatternVote
}

func sumNodesInView(layerBlockCounter map[mesh.LayerID]int, layer mesh.LayerID, pLayer mesh.LayerID) vec {
	var sum int
	for sum = 0; layer <= pLayer; layer++ {
		sum = sum + layerBlockCounter[layer]
	}
	return Against.Multiply(sum)
}

func (ni *ninjaTortoise) processBlocks(layer *mesh.Layer) {
	for _, block := range layer.Blocks() {
		ni.processBlock(block)
	}
}

func (ni *ninjaTortoise) handleGenesis(genesis *mesh.Layer) {
	blkIds := make([]mesh.BlockID, 0, len(genesis.Blocks()))
	for _, blk := range genesis.Blocks() {
		blkIds = append(blkIds, blk.ID())
	}
	vp := votingPattern{id: getId(blkIds), LayerID: Genesis}
	ni.pBase = vp
	ni.tGood[Genesis] = vp
	ni.tExplicit[genesis.Blocks()[0].ID()] = make(map[mesh.LayerID]votingPattern, K*ni.avgLayerSize)
}

//todo send map instead of ni
func updatePatSupport(ni *ninjaTortoise, p votingPattern, bids []mesh.BlockID, idx mesh.LayerID) {
	if val, found := ni.tPatSupport[p]; !found || val == nil {
		ni.tPatSupport[p] = make(map[mesh.LayerID]votingPattern)
	}
	pid := getId(bids)
	ni.Debug("update support for %d layer %d supported pattern %d", p, idx, pid)
	ni.tPatSupport[p][idx] = votingPattern{id: pid, LayerID: idx}
}

func initTallyToBase(tally map[votingPattern]map[mesh.BlockID]vec, base votingPattern, p votingPattern) {
	if _, found := tally[p]; !found {
		tally[p] = make(map[mesh.BlockID]vec)
	}
	for k, v := range tally[base] {
		tally[p][k] = v
	}
}

func (ni *ninjaTortoise) latestComplete() mesh.LayerID {
	return ni.pBase.Layer()
}

func (ni *ninjaTortoise) getVotes() map[mesh.BlockID]vec {
	return ni.tVote[ni.pBase]
}

func (ni *ninjaTortoise) getVote(id mesh.BlockID) vec {
	block, err := ni.GetBlock(id)
	if err != nil {
		panic(fmt.Sprintf("error block not found ID %d", id))
	}

	if block.Layer() > ni.pBase.Layer() {
		ni.Error("we dont have an opinion on block according to current pbase")
		return Against
	}

	return ni.tVote[ni.pBase][id]
}

func (ni *ninjaTortoise) handleIncomingLayer(newlyr *mesh.Layer) { //i most recent layer
	ni.Info("update tables layer %d with %d blocks", newlyr.Index(), len(newlyr.Blocks()))

	ni.processBlocks(newlyr)

	if newlyr.Index() == Genesis {
		ni.handleGenesis(newlyr)
		return
	}

	l := ni.findMinimalNewlyGoodLayer(newlyr)

	//from minimal newly good pattern to current layer
	//update pattern tally for all good layers
	for j := l; j > 0 && j < newlyr.Index(); j++ {
		if p, gfound := ni.tGood[j]; gfound {
			//init p's tally to pBase tally
			initTallyToBase(ni.tTally, ni.pBase, p)

			//find bottom of window
			var windowStart mesh.LayerID
			if Window > newlyr.Index() {
				windowStart = 0
			} else {
				windowStart = newlyr.Index() - Window + 1
			}

			view := make(map[mesh.BlockID]struct{})
			lCntr := make(map[mesh.LayerID]int)
			correctionMap, effCountMap, getCrrEffCnt := ni.getCorrEffCounter()
			foo := func(block *mesh.BlockHeader) {
				view[block.ID()] = struct{}{} //all blocks in view
				for _, id := range block.BlockVotes {
					view[id] = struct{}{}
				}
				lCntr[block.Layer()]++ //amount of blocks for each layer in view
				getCrrEffCnt(block)    //calc correction and eff count
			}

			ni.ForBlockInView(ni.tPattern[p], ni.pBase.Layer()+1, foo, func(err error) {})

			//add corrected implicit votes
			ni.updatePatternTally(p, windowStart, correctionMap, effCountMap)

			//add explicit votes
			addPtrnVt := ni.addPatternVote(p, view)
			for bl := range view {
				addPtrnVt(bl)
			}

			complete := true
			for idx := windowStart; idx < j; idx++ {
				layer, _ := ni.LayerBlockIds(idx) //todo handle error
				bids := make([]mesh.BlockID, 0, ni.avgLayerSize)
				for _, bid := range layer {
					//if bid is not in p's view.
					//add negative vote multiplied by the amount of blocks in the view
					//explicit votes against (not in view )
					if _, found := view[bid]; idx >= ni.pBase.Layer() && !found {
						ni.tTally[p][bid] = sumNodesInView(lCntr, idx+1, p.Layer())
					}

					if val, found := ni.tVote[p]; !found || val == nil {
						ni.tVote[p] = make(map[mesh.BlockID]vec)
					}

					if vote := globalOpinion(ni.tTally[p][bid], ni.avgLayerSize, float64(p.LayerID-idx)); vote != Abstain {
						ni.tVote[p][bid] = vote
						if vote == Support {
							bids = append(bids, bid)
						}
					} else {
						ni.tVote[p][bid] = vote
						complete = false //not complete
					}
				}
				updatePatSupport(ni, p, bids, idx)
			}

			//update correction vectors after vote count
			ni.updateCorrectionVectors(p, windowStart)

			// update completeness of p
			if _, found := ni.tComplete[p]; complete && !found {
				ni.tComplete[p] = struct{}{}
				ni.pBase = p
				ni.Debug("found new complete and good pattern for layer %d pattern %d with %d support ", l, p.id, ni.tSupport[p])
			}
		}
	}
	ni.Info("finished layer %d pbase is %d", newlyr.Index(), ni.pBase.Layer())
	ni.evictOutOfWindow(newlyr.Index())
	return
}
