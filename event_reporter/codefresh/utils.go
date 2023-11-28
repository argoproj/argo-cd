package codefresh

import (
	"fmt"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

// Backoff for an operation
type Backoff struct {
	// The initial duration in nanoseconds or strings like "1s", "3m"
	// +optional
	Duration *Int64OrString `json:"duration,omitempty" protobuf:"bytes,1,opt,name=duration"`
	// Duration is multiplied by factor each iteration
	// +optional
	Factor *Amount `json:"factor,omitempty" protobuf:"bytes,2,opt,name=factor"`
	// The amount of jitter applied each iteration
	// +optional
	Jitter *Amount `json:"jitter,omitempty" protobuf:"bytes,3,opt,name=jitter"`
	// Exit with error after this many steps
	// +optional
	Steps int32 `json:"steps,omitempty" protobuf:"varint,4,opt,name=steps"`
}

// Amount represent a numeric amount.
type Amount struct {
	Value []byte `json:"value" protobuf:"bytes,1,opt,name=value"`
}

type Int64OrString struct {
	Type     Type   `json:"type" protobuf:"varint,1,opt,name=type,casttype=Type"`
	Int64Val int64  `json:"int64Val,omitempty" protobuf:"varint,2,opt,name=int64Val"`
	StrVal   string `json:"strVal,omitempty" protobuf:"bytes,3,opt,name=strVal"`
}

// Type represents the stored type of Int64OrString.
type Type int64

const (
	Int64  Type = iota // The Int64OrString holds an int64.
	String             // The Int64OrString holds a string.
)

func NewAmount(s string) Amount {
	return Amount{Value: []byte(s)}
}

// FromString creates an Int64OrString object with a string value.
func FromString(val string) Int64OrString {
	return Int64OrString{Type: String, StrVal: val}
}

// Int64Value returns the Int64Val if type Int64, or if
// it is a String, will attempt a conversion to int64,
// returning 0 if a parsing error occurs.
func (int64str *Int64OrString) Int64Value() int64 {
	if int64str.Type == String {
		i, _ := strconv.ParseInt(int64str.StrVal, 10, 64)
		return i
	}
	return int64str.Int64Val
}

var (
	defaultFactor   = NewAmount("1.0")
	defaultJitter   = NewAmount("1")
	defaultDuration = FromString("1s")

	DefaultBackoff = Backoff{
		Steps:    5,
		Duration: &defaultDuration,
		Factor:   &defaultFactor,
		Jitter:   &defaultJitter,
	}
)

func (n *Amount) Float64() (float64, error) {
	return strconv.ParseFloat(string(n.Value), 64)
}

// Convert2WaitBackoff converts to a wait backoff option
func Convert2WaitBackoff(backoff *Backoff) (*wait.Backoff, error) {
	result := wait.Backoff{}

	d := backoff.Duration
	if d == nil {
		d = &defaultDuration
	}
	if d.Type == Int64 {
		result.Duration = time.Duration(d.Int64Value())
	} else {
		parsedDuration, err := time.ParseDuration(d.StrVal)
		if err != nil {
			return nil, err
		}
		result.Duration = parsedDuration
	}

	factor := backoff.Factor
	if factor == nil {
		factor = &defaultFactor
	}
	f, err := factor.Float64()
	if err != nil {
		return nil, fmt.Errorf("invalid factor, %w", err)
	}
	result.Factor = f

	jitter := backoff.Jitter
	if jitter == nil {
		jitter = &defaultJitter
	}
	j, err := jitter.Float64()
	if err != nil {
		return nil, fmt.Errorf("invalid jitter, %w", err)
	}
	result.Jitter = j

	if backoff.Steps > 0 {
		result.Steps = backoff.GetSteps()
	} else {
		result.Steps = int(DefaultBackoff.Steps)
	}
	return &result, nil
}

func (b Backoff) GetSteps() int {
	return int(b.Steps)
}

func WithRetry(backoff *Backoff, f func() error) error {
	if backoff == nil {
		backoff = &DefaultBackoff
	}
	b, err := Convert2WaitBackoff(backoff)
	if err != nil {
		return fmt.Errorf("invalid backoff configuration, %w", err)
	}
	_ = wait.ExponentialBackoff(*b, func() (bool, error) {
		if err = f(); err != nil {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("failed after retries: %w", err)
	}
	return nil
}
