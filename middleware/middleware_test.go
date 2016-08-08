package middleware

import (
	"testing"

	m "github.com/linkedin-inc/mane/model"
	u "github.com/linkedin-inc/mane/util"
)

var contexts []*m.SMSContext

func init() {

}

type KeepOdd struct {
	ActionName string
}

func (m *KeepOdd) Name() string {
	return m.ActionName
}

func (m *KeepOdd) Call(context *m.SMSContext, next func() bool) bool {
	if u.Atoi64(context.Phone)%2 == 1 {
		next()
		return true
	} else {
		return false
	}
}

func NewKeepOdd(name string) *KeepOdd {
	return &KeepOdd{ActionName: name}
}

type KeepThree struct {
	ActionName string
}

func (m *KeepThree) Name() string {
	return m.ActionName
}

func (m *KeepThree) Call(context *m.SMSContext, next func() bool) bool {
	if u.Atoi64(context.Phone)%3 == 0 {
		next()
		return true
	} else {
		return false
	}
}

func NewKeepThree(name string) *KeepThree {
	return &KeepThree{ActionName: name}
}

type PanicZero struct{}

func (m *PanicZero) Name() string {
	return "PanicZero"
}

func (m *PanicZero) Call(context *m.SMSContext, next func() bool) bool {
	if u.Atoi64(context.Phone) == 0 {
		panic("WTF")
	}
	next()
	return true
}

func NewPanicZero() *PanicZero {
	return &PanicZero{}
}

func TestMiddleware_Append(t *testing.T) {
	t.Log("\n\n")
	// just create some fake contexts
	N := 10
	for i := 0; i < N; i++ {
		contexts = append(contexts, m.NewSMSContext(u.Itoa(i), "", nil))
	}
	allowedContexts := NewMiddleware(NewKeepOdd("KeepOdd")).Call(contexts)
	if len(allowedContexts) != 5 {
		t.Error("TestMiddleware_Append failed")
		t.Logf("allowedContexts:%v\n", len(allowedContexts))
	}
}

// nil actions should always return true
func TestNewMiddleware(t *testing.T) {
	t.Log("\n\n")
	contexts = contexts[:0]
	N := 10
	for i := 0; i < N; i++ {
		contexts = append(contexts, m.NewSMSContext(u.Itoa(i), "", nil))
	}
	allowedContexts := NewMiddleware().Call(contexts)
	if len(allowedContexts) != 10 {
		t.Errorf("TestNewMiddleware TestNewMiddleware failed. allowedContexts:%v\n", len(allowedContexts))
	}
}

// 0 3 6 9
func TestMiddleware_Append2(t *testing.T) {
	t.Log("\n\n")
	contexts = contexts[:0]
	N := 10
	for i := 0; i < N; i++ {
		contexts = append(contexts, m.NewSMSContext(u.Itoa(i), "", nil))
	}
	allowedContexts := NewMiddleware(NewKeepThree("KeepThree")).Call(contexts)
	if len(allowedContexts) != 4 {
		t.Errorf("TestNewMiddleware failed. allowedContexts:%v,%v\n", len(allowedContexts), allowedContexts)
	} else {
		t.Logf("TestMiddleware_Append2 allowedContexts:%v\n", allowedContexts)
	}
}

// 3 9
func TestMiddleware_Append3(t *testing.T) {
	t.Log("\n\n")
	contexts = contexts[:0]
	N := 10
	for i := 0; i < N; i++ {
		contexts = append(contexts, m.NewSMSContext(u.Itoa(i), "", nil))
	}
	allowedContexts := NewMiddleware(NewKeepOdd("KeepOdd"), NewKeepThree("KeepThree")).Call(contexts)
	if len(allowedContexts) != 2 {
		t.Errorf("TestNewMiddleware failed. allowedContexts:%v,%v\n", len(allowedContexts), allowedContexts)
	} else {
		t.Logf("TestMiddleware_Append3 allowedContexts:%v\n", allowedContexts)
	}
}

//  3 6 9
func TestMiddleware_Append4(t *testing.T) {
	t.Log("\n\n")
	contexts = contexts[:0]
	N := 10
	for i := 0; i < N; i++ {
		contexts = append(contexts, m.NewSMSContext(u.Itoa(i), "", nil))
	}
	allowedContexts := NewMiddleware(NewKeepThree("KeepThree"), NewPanicZero()).Call(contexts)
	if len(allowedContexts) != 3 {
		t.Errorf("TestNewMiddleware failed. allowedContexts:%v,%v\n", len(allowedContexts), allowedContexts)
	} else {
		t.Logf("TestMiddleware_Append2 allowedContexts:%v\n", allowedContexts)
	}
}
