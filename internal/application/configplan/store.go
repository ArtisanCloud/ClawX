package configplan

import "errors"

var ErrPlanNotFound = errors.New("config plan not found")

type Store interface {
	Set(conversationID string, plan Plan) error
	Get(conversationID string) (Plan, bool)
	Pop(conversationID string) (Plan, bool)
	Delete(conversationID string) bool
}
