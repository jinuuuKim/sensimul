package sim

import (
	"math/rand"
	"time"
)

type Rand struct {
	rng *rand.Rand
}

func NewRand(seed int64) *Rand {
	return &Rand{
		rng: rand.New(rand.NewSource(seed)),
	}
}

func (r *Rand) Float64() float64 {
	return r.rng.Float64()
}

func (r *Rand) Int63() int64 {
	return r.rng.Int63()
}

func (r *Rand) Uint64() uint64 {
	return uint64(r.rng.Int63())
}

func (r *Rand) Float64n(n float64) float64 {
	return r.rng.Float64() * n
}

func (r *Rand) Int63n(n int64) int64 {
	return r.rng.Int63n(n)
}

func (r *Rand) Seed(seed int64) {
	r.rng = rand.New(rand.NewSource(seed))
}

func NowFunc() time.Time {
	return time.Now()
}

func SinceFunc(t time.Time) time.Duration {
	return time.Since(t)
}
