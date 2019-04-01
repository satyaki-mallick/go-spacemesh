package hare

import (
	"encoding/binary"
	"fmt"
	"github.com/spacemeshos/go-spacemesh/hare/metrics"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/signing"
	"hash/fnv"
)

// Tracks notify messages
type NotifyTracker struct {
	notifies     map[string]struct{} // tracks PubKey->Notification
	tracker      *RefCountTracker    // tracks ref count to each seen set
	certificates map[uint32]struct{} // tracks Set->certificate
}

func NewNotifyTracker(expectedSize int) *NotifyTracker {
	nt := &NotifyTracker{}
	nt.notifies = make(map[string]struct{}, expectedSize)
	nt.tracker = NewRefCountTracker()
	nt.certificates = make(map[uint32]struct{}, expectedSize)

	return nt
}

// Track the provided notification message
// Returns true if the message didn't affect the state, false otherwise
func (nt *NotifyTracker) OnNotify(msg *Msg) bool {
	verifier, err := signing.NewVerifier(msg.PubKey)
	if err != nil {
		log.Warning("Could not construct verifier: ", err)
		return true
	}

	if _, exist := nt.notifies[verifier.String()]; exist { // already seenSenders
		return true // ignored
	}

	// keep msg for pub
	nt.notifies[verifier.String()] = struct{}{}

	// track that set
	s := NewSet(msg.Message.Values)
	nt.onCertificate(msg.Message.Cert.AggMsgs.Messages[0].Message.K, s)
	nt.tracker.Track(s.Id())
	metrics.NotifyCounter.With("set_id", fmt.Sprint(s.Id())).Add(1)

	return false
}

// Returns the notification count tracked for the provided set
func (nt *NotifyTracker) NotificationsCount(s *Set) int {
	return int(nt.tracker.CountStatus(s.Id()))
}

func calcId(k int32, set *Set) uint32 {
	hash := fnv.New32()

	// write k
	buff := make([]byte, 4)
	binary.LittleEndian.PutUint32(buff, uint32(k)) // k>=0 because this not pre-round
	hash.Write(buff)

	// write set objectId
	buff = make([]byte, 4)
	binary.LittleEndian.PutUint32(buff, uint32(set.Id()))
	hash.Write(buff)

	return hash.Sum32()
}

func (nt *NotifyTracker) onCertificate(k int32, set *Set) {
	nt.certificates[calcId(k, set)] = struct{}{}
}

// Checks if a certificates exist for the provided set in round k
func (nt *NotifyTracker) HasCertificate(k int32, set *Set) bool {
	_, exist := nt.certificates[calcId(k, set)]
	return exist
}
