package hare

import (
	"encoding/binary"
	"github.com/stretchr/testify/assert"
	"math"
	"math/rand"
	"testing"
	"time"
)

const numOfClients = 100

func TestMockHashOracle_Register(t *testing.T) {
	oracle := NewMockHashOracle(numOfClients)
	oracle.Register(generateSigning(t).Verifier().String())
	oracle.Register(generateSigning(t).Verifier().String())
	assert.Equal(t, 2, len(oracle.clients))
}

func TestMockHashOracle_Unregister(t *testing.T) {
	oracle := NewMockHashOracle(numOfClients)
	pub := generateSigning(t)
	oracle.Register(pub.Verifier().String())
	assert.Equal(t, 1, len(oracle.clients))
	oracle.Unregister(pub.Verifier().String())
	assert.Equal(t, 0, len(oracle.clients))
}

func TestMockHashOracle_Concurrency(t *testing.T) {
	oracle := NewMockHashOracle(numOfClients)
	c := make(chan Signing, 1000)
	done := make(chan int, 2)

	go func() {
		for i := 0; i < 500; i++ {
			pub := generateSigning(t)
			oracle.Register(pub.Verifier().String())
			c <- pub
		}
		done <- 1
	}()

	go func() {
		for i := 0; i < 400; i++ {
			s := <-c
			oracle.Unregister(s.Verifier().String())
		}
		done <- 1
	}()

	<-done
	<-done
	assert.Equal(t, len(oracle.clients), 100)
}

func genSig() Signature {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	sig := make([]byte, 4, 4)
	binary.LittleEndian.PutUint32(sig, r1.Uint32())

	return sig[:]
}

func TestMockHashOracle_Role(t *testing.T) {
	oracle := NewMockHashOracle(numOfClients)
	for i := 0; i < numOfClients; i++ {
		pub := generateSigning(t)
		oracle.Register(pub.Verifier().String())
	}

	committeeSize := 20
	counter := 0
	for i := 0; i < numOfClients; i++ {
		if oracle.Eligible(instanceId1, 0, committeeSize, generateSigning(t).Verifier().String(), []byte(genSig())) {
			counter++
		}
	}

	if counter*3 < committeeSize { // allow only deviation
		t.Errorf("Comity size error. Expected: %v Actual: %v", committeeSize, counter)
		t.Fail()
	}
}

func TestMockHashOracle_calcThreshold(t *testing.T) {
	oracle := NewMockHashOracle(2)
	oracle.Register(generateSigning(t).Verifier().String())
	oracle.Register(generateSigning(t).Verifier().String())
	assert.Equal(t, uint32(math.MaxUint32/2), oracle.calcThreshold(1))
	assert.Equal(t, uint32(math.MaxUint32), oracle.calcThreshold(2))
}

func TestFixedRolacle_Eligible(t *testing.T) {
	oracle := newFixedRolacle(numOfClients)
	for i := 0; i < numOfClients-1; i++ {
		oracle.Register(NewMockSigning().Verifier().String())
	}
	v := NewMockSigning().Verifier()
	oracle.Register(v.String())

	res := oracle.Eligible(nil, 1, 10, v.String(), nil)
	assert.True(t, res == oracle.Eligible(nil, 1, 10, v.String(), nil))
}

func TestFixedRolacle_Eligible2(t *testing.T) {
	pubs := make([]string, 0, numOfClients)
	oracle := newFixedRolacle(numOfClients)
	for i := 0; i < numOfClients; i++ {
		s := NewMockSigning().Verifier().String()
		pubs = append(pubs, s)
		oracle.Register(s)
	}

	count := 0
	for _, p := range pubs {
		if oracle.Eligible(nil, 1, 10, p, nil) {
			count++
		}
	}

	assert.Equal(t, 10, count)

	count = 0
	for _, p := range pubs {
		if oracle.Eligible(nil, 1, 20, p, nil) {
			count++
		}
	}

	assert.Equal(t, 10, count)
}

func TestFixedRolacle_Eligible3(t *testing.T) {
	oracle := newFixedRolacle(2)
	s1 := NewMockSigning().Verifier().String()
	oracle.Register(s1)

	s2 := NewMockSigning().Verifier().String()
	oracle.Register(s2)

	res1 := true
	res2 := true
	for i := 0;i<10;i++ {
		res1 = res1 != oracle.Eligible(nil, i, 1, s1, nil)
		res2 = res2 != oracle.Eligible(nil, i, 1, s2, nil)
	}

	assert.False(t, res1)
	assert.False(t, res2)
}
