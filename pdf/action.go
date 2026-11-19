// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"fmt"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/pdfwriter"
)

// This file ports krilla src/interactive/action.rs. The actions ResetForm
// and SubmitForm, with ResetFormAction, SubmitFormAction,
// ActionFieldFilter, SubmissionFormat and HttpMethod, are for the forms,
// which the base port leaves out (D62).
//
// PDF actions, allowing you to add interactivity to the document.
//
// PDF has the concept of "actions", which encompass things like navigating
// to a URL, opening some file on the system, and so on. The PDF reference
// defines a whole bunch of actions, but krilla does not expose nearly all of
// them, and never will.

// Action is Action in krilla src/interactive/action.rs. A Rust enum with
// data becomes an interface with one type for each case: LinkAction and
// GotoAction. An Action is a Target, as the case Target::Action of upstream
// makes it.
//
// A type of action.
type Action interface {
	Target
	// serializeAction is Action::serialize. It writes the action into the
	// dictionary of an action.
	serializeAction(sc *serializeContext, action *pdfwriter.Action, location Location) error
}

// LinkAction is LinkAction in krilla src/interactive/action.rs.
//
// A link action. Will open a link when clicked.
type LinkAction struct {
	uri string
}

// NewLinkAction is LinkAction::new in krilla src/interactive/action.rs.
//
// Create a new link action that will open a URI when clicked.
func NewLinkAction(uri string) LinkAction {
	return LinkAction{uri: uri}
}

// serializeAction is the case Action::Link of Action::serialize and
// LinkAction::serialize in krilla src/interactive/action.rs.
func (l LinkAction) serializeAction(_ *serializeContext, action *pdfwriter.Action, _ Location) error {
	action.ActionType(pdfwriter.ActionTypeURI).URI(pdfwriter.Str(l.uri))
	return nil
}

// serializeTarget is the case Target::Action of
// LinkAnnotation::serialize_type in krilla src/interactive/annotation.rs.
func (l LinkAction) serializeTarget(sc *serializeContext, a *pdfwriter.Annotation, location Location) error {
	return serializeActionTarget(sc, l, a, location)
}

// GotoAction is the case Action::Goto in krilla src/interactive/action.rs.
//
// A go-to action.
type GotoAction struct {
	destination Destination
}

// NewGotoAction makes the case Action::Goto in krilla
// src/interactive/action.rs. A nil destination makes Document.Finish return
// ErrPDFInvalidAnnotation.
func NewGotoAction(destination Destination) GotoAction {
	return GotoAction{destination: destination}
}

// serializeAction is the case Action::Goto of Action::serialize in krilla
// src/interactive/action.rs.
func (g GotoAction) serializeAction(sc *serializeContext, action *pdfwriter.Action, _ Location) error {
	if g.destination == nil {
		return fmt.Errorf("%w: a go-to action has no destination", okage.ErrPDFInvalidAnnotation)
	}
	destEntry := action.ActionType(pdfwriter.ActionTypeGoTo).Insert(pdfwriter.Name("D"))
	return g.destination.serializeDestination(sc, destEntry)
}

// serializeTarget is the case Target::Action of
// LinkAnnotation::serialize_type in krilla src/interactive/annotation.rs.
func (g GotoAction) serializeTarget(sc *serializeContext, a *pdfwriter.Annotation, location Location) error {
	return serializeActionTarget(sc, g, a, location)
}

// serializeActionTarget writes an action into the key /A of an annotation,
// as LinkAnnotation::serialize_type in krilla src/interactive/annotation.rs
// does for the case Target::Action.
func serializeActionTarget(sc *serializeContext, action Action, a *pdfwriter.Annotation, location Location) error {
	w := a.Action()
	if err := action.serializeAction(sc, w, location); err != nil {
		return err
	}
	w.Finish()
	return nil
}
