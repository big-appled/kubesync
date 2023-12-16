package predicatebase

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type IgnoreAllPredicate struct {
	predicate.Funcs
}

// Create implements Predicate.
func (IgnoreAllPredicate) Create(_ event.CreateEvent) bool {
	return false
}

// Delete implements Predicate.
func (IgnoreAllPredicate) Delete(_ event.DeleteEvent) bool {
	return false
}

// Update implements Predicate.
func (IgnoreAllPredicate) Update(_ event.UpdateEvent) bool {
	return false
}

// Generic implements Predicate.
func (IgnoreAllPredicate) Generic(_ event.GenericEvent) bool {
	return false
}
