package filter

import (
	"linkedin/service/mongodb"

	m "github.com/linkedin-inc/mane/model"
	t "github.com/linkedin-inc/mane/template"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type UnsubscribeFilter struct {
	Type Type `bson:"type" json:"type"`
}

func NewUnsubscribeFilter() *UnsubscribeFilter {
	return &UnsubscribeFilter{
		Type: FilterTypeUnsubscribe,
	}
}

func (f *UnsubscribeFilter) Allow(phone string, template t.Name) bool {
	//ignore template
	existed := mongodb.Exec(m.CollUnsubscriber, func(c *mgo.Collection) error {
		return c.Find(bson.M{"phone": phone}).One(nil)
	})
	return !existed
}

func (f *UnsubscribeFilter) WhichType() Type {
	return f.Type
}

func (f *UnsubscribeFilter) Apply(strategies []Strategy) {
	//do nothing
}

func (f *UnsubscribeFilter) Resolve(expression string) (interface{}, error) {
	//do nothing
	return nil, nil
}
