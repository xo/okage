// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"fmt"
)

// This file ports the part of pdf-writer src/actions.rs that krilla calls
// for a link: the writer Action and the types of actions.
//
// krilla writes the fields, the form flags and the additional actions only
// for the forms, and no rendition, so the writers Fields and
// AdditionalActions, the types FormActionFlags and RenditionOperation, and
// the methods that write them or a file specification have no port.

// Action is Action in pdf-writer src/actions.rs.
//
// Writer for an action dictionary.
//
// This struct is created by Annotation.Action.
type Action struct {
	*Dict
}

// start is the macro writer! for Action in pdf-writer src/actions.rs.
func (a *Action) start(obj Obj) {
	a.Dict = obj.Dict()
	a.Pair(Name("Type"), Name("Action"))
}

// ActionType is Action::action_type in pdf-writer src/actions.rs.
//
// Write the /S attribute to set the action type.
func (a *Action) ActionType(kind ActionType) *Action {
	a.Pair(Name("S"), kind.toName())
	return a
}

// Destination is Action::destination in pdf-writer src/actions.rs.
//
// Start writing the /D attribute to set the destination of this GoTo-type
// action.
func (a *Action) Destination() *Destination {
	return Start[Destination](a.Insert(Name("D")))
}

// DestinationNamed is Action::destination_named in pdf-writer
// src/actions.rs.
//
// Write the /D attribute to set the destination of this GoTo-type action to
// a named destination.
func (a *Action) DestinationNamed(name Name) *Action {
	a.Pair(Name("D"), name)
	return a
}

// NewWindow is Action::new_window in pdf-writer src/actions.rs.
//
// Write the /NewWindow attribute to set whether this remote GoTo action
// should open the referenced destination in another window.
func (a *Action) NewWindow(newWindow bool) *Action {
	a.Pair(Name("NewWindow"), Bool(newWindow))
	return a
}

// URI is Action::uri in pdf-writer src/actions.rs.
//
// Write the /URI attribute to set where this link action goes.
func (a *Action) URI(uri Str) *Action {
	a.Pair(Name("URI"), uri)
	return a
}

// IsMap is Action::is_map in pdf-writer src/actions.rs.
//
// Write the /IsMap attribute to set if the click position of the user's
// cursor inside the link rectangle should be appended to the referenced URI
// as a query parameter.
func (a *Action) IsMap(isMap bool) *Action {
	a.Pair(Name("IsMap"), Bool(isMap))
	return a
}

// JSString is Action::js_string in pdf-writer src/actions.rs.
//
// Write the /JS attribute to set the script of this action as a text
// string. Only permissible for JavaScript and Rendition actions.
func (a *Action) JSString(script TextStr) *Action {
	a.Pair(Name("JS"), script)
	return a
}

// JSStream is Action::js_stream in pdf-writer src/actions.rs.
//
// Write the /JS attribute to set the script of this action as a text
// stream. The indirect reference shall point to a stream containing valid
// ECMAScript. The stream must have PdfDocEncoding or be in Unicode, starting
// with U+FEFF. Only permissible for JavaScript and Rendition actions.
func (a *Action) JSStream(script Ref) *Action {
	a.Pair(Name("JS"), script)
	return a
}

// Annotation is Action::annotation in pdf-writer src/actions.rs.
//
// Write the /AN attribute to provide a reference to the screen annotation
// for the operation. Required if OP is present.
func (a *Action) Annotation(id Ref) *Action {
	a.Pair(Name("AN"), id)
	return a
}

// ActionType is ActionType in pdf-writer src/actions.rs.
//
// What kind of action to perform when clicking a link annotation.
type ActionType uint8

// The values of ActionType.
const (
	// ActionTypeGoTo goes to a destination in the document.
	ActionTypeGoTo ActionType = iota
	// ActionTypeRemoteGoTo goes to a destination in another document.
	ActionTypeRemoteGoTo
	// ActionTypeLaunch launches an application.
	//
	// This action type is forbidden in PDF/A.
	ActionTypeLaunch
	// ActionTypeURI opens a URI.
	ActionTypeURI
	// ActionTypeSubmitForm sets an annotation's hidden flag. PDF 1.2+.
	ActionTypeSubmitForm
	// ActionTypeResetForm sets form fields to their default values. PDF
	// 1.2+.
	//
	// This action type is forbidden in PDF/A.
	ActionTypeResetForm
	// ActionTypeImportData imports form field values from a file. PDF 1.2+.
	//
	// This action type is forbidden in PDF/A.
	ActionTypeImportData
	// ActionTypeJavaScript executes a JavaScript action. PDF 1.2+.
	//
	// See Adobe's JavaScript for Acrobat API Reference
	// (https://opensource.adobe.com/dc-acrobat-sdk-docs/acrobatsdk/pdfs/acrobatsdk_jsapiref.pdf)
	// and ISO 21757.
	//
	// This action type is forbidden in PDF/A.
	ActionTypeJavaScript
	// ActionTypeRendition is a rendition action to control the playing of
	// multimedia content. PDF 1.5+.
	//
	// This action type is forbidden in PDF/A.
	ActionTypeRendition
)

// actionTypeNames holds the Rust name of each value of ActionType.
var actionTypeNames = [...]string{
	ActionTypeGoTo:       "GoTo",
	ActionTypeRemoteGoTo: "RemoteGoTo",
	ActionTypeLaunch:     "Launch",
	ActionTypeURI:        "Uri",
	ActionTypeSubmitForm: "SubmitForm",
	ActionTypeResetForm:  "ResetForm",
	ActionTypeImportData: "ImportData",
	ActionTypeJavaScript: "JavaScript",
	ActionTypeRendition:  "Rendition",
}

// String returns the Rust name of the type.
func (t ActionType) String() string {
	if int(t) < len(actionTypeNames) {
		return actionTypeNames[t]
	}
	return "ActionType(?)"
}

// toName is ActionType::to_name in pdf-writer src/actions.rs.
func (t ActionType) toName() Name {
	switch t {
	case ActionTypeGoTo:
		return Name("GoTo")
	case ActionTypeRemoteGoTo:
		return Name("GoToR")
	case ActionTypeLaunch:
		return Name("Launch")
	case ActionTypeURI:
		return Name("URI")
	case ActionTypeSubmitForm:
		return Name("SubmitForm")
	case ActionTypeResetForm:
		return Name("ResetForm")
	case ActionTypeImportData:
		return Name("ImportData")
	case ActionTypeJavaScript:
		return Name("JavaScript")
	case ActionTypeRendition:
		return Name("Rendition")
	}
	panic(fmt.Sprintf("pdfwriter: the action type %d is not known", uint8(t)))
}

// FileSpec is Action::file_spec in pdf-writer src/actions.rs.
//
// Start writing the /F attribute.
func (a *Action) FileSpec() *FileSpec {
	return Start[FileSpec](a.Insert(Name("F")))
}
