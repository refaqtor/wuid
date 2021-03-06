/*
Package wuid provides WUID, an extremely fast unique number generator. It is 10-135 times faster
than UUID and 4600 times faster than generating unique numbers with Redis.

WUID generates unique 64-bit integers in sequence. The high 28 bits are loaded from a data store.
By now, Redis, MySQL, and MongoDB are supported.
*/
package wuid

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/edwingeng/wuid/internal"
)

/*
Logger includes internal.Logger, while internal.Logger includes:
	Info(args ...interface{})
	Warn(args ...interface{})
*/
type Logger interface {
	internal.Logger
}

// WUID is an extremely fast unique number generator.
type WUID struct {
	w *internal.WUID
}

// NewWUID creates a new WUID instance.
func NewWUID(tag string, logger Logger, opts ...Option) *WUID {
	var opts2 []internal.Option
	for _, opt := range opts {
		opts2 = append(opts2, internal.Option(opt))
	}
	return &WUID{w: internal.NewWUID(tag, logger, opts2...)}
}

// Next returns the next unique number.
func (this *WUID) Next() uint64 {
	return this.w.Next()
}

type H28Callback func() (h28 uint64, done func(), err error)

// LoadH28WithCallback calls cb to get a number, and then sets it as the high 28 bits of the unique
// numbers that Next generates.
// The number returned by cb should look like 0x000123, not 0x0001230000000000.
func (this *WUID) LoadH28WithCallback(cb H28Callback) error {
	if cb == nil {
		return errors.New("cb cannot be nil. tag: " + this.w.Tag)
	}

	h28, done, err := cb()
	if err != nil {
		return err
	}
	if done != nil {
		defer func() {
			done()
		}()
	}

	if err = this.w.VerifyH28(h28); err != nil {
		return err
	}
	if this.w.Section == 0 {
		if h28 == atomic.LoadUint64(&this.w.N)>>36 {
			return fmt.Errorf("the h28 should be a different value other than %d. tag: %s", h28, this.w.Tag)
		}
	} else {
		if h28 == (atomic.LoadUint64(&this.w.N)>>36)&0x0FFFFF {
			return fmt.Errorf("the h28 should be a different value other than %d. tag: %s", h28, this.w.Tag)
		}
	}

	this.w.Reset(h28 << 36)
	this.w.Logger.Info(fmt.Sprintf("<wuid> new h28: %d. tag: %s", h28, this.w.Tag))

	this.w.Lock()
	defer this.w.Unlock()

	if this.w.Renew != nil {
		return nil
	}
	this.w.Renew = func() error {
		return this.LoadH28WithCallback(cb)
	}

	return nil
}

// RenewNow reacquires the high 28 bits from your data store immediately
func (this *WUID) RenewNow() error {
	return this.w.RenewNow()
}

// Option should never be used directly.
type Option internal.Option

// WithSection adds a section ID to the generated numbers. The section ID must be in between [1, 15].
// It occupies the highest 4 bits of the numbers.
func WithSection(section uint8) Option {
	return Option(internal.WithSection(section))
}

// WithH28Verifier sets your own h28 verifier
func WithH28Verifier(cb func(h28 uint64) error) Option {
	return Option(internal.WithH28Verifier(cb))
}
