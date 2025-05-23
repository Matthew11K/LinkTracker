// Code generated by ogen, DO NOT EDIT.

package v1_bot

import (
	"net/url"
)

// Ref: #/components/schemas/ApiErrorResponse
type ApiErrorResponse struct {
	Description      OptString `json:"description"`
	Code             OptString `json:"code"`
	ExceptionName    OptString `json:"exceptionName"`
	ExceptionMessage OptString `json:"exceptionMessage"`
	Stacktrace       []string  `json:"stacktrace"`
}

// GetDescription returns the value of Description.
func (s *ApiErrorResponse) GetDescription() OptString {
	return s.Description
}

// GetCode returns the value of Code.
func (s *ApiErrorResponse) GetCode() OptString {
	return s.Code
}

// GetExceptionName returns the value of ExceptionName.
func (s *ApiErrorResponse) GetExceptionName() OptString {
	return s.ExceptionName
}

// GetExceptionMessage returns the value of ExceptionMessage.
func (s *ApiErrorResponse) GetExceptionMessage() OptString {
	return s.ExceptionMessage
}

// GetStacktrace returns the value of Stacktrace.
func (s *ApiErrorResponse) GetStacktrace() []string {
	return s.Stacktrace
}

// SetDescription sets the value of Description.
func (s *ApiErrorResponse) SetDescription(val OptString) {
	s.Description = val
}

// SetCode sets the value of Code.
func (s *ApiErrorResponse) SetCode(val OptString) {
	s.Code = val
}

// SetExceptionName sets the value of ExceptionName.
func (s *ApiErrorResponse) SetExceptionName(val OptString) {
	s.ExceptionName = val
}

// SetExceptionMessage sets the value of ExceptionMessage.
func (s *ApiErrorResponse) SetExceptionMessage(val OptString) {
	s.ExceptionMessage = val
}

// SetStacktrace sets the value of Stacktrace.
func (s *ApiErrorResponse) SetStacktrace(val []string) {
	s.Stacktrace = val
}

func (*ApiErrorResponse) updatesPostRes() {}

// Ref: #/components/schemas/LinkUpdate
type LinkUpdate struct {
	ID          OptInt64  `json:"id"`
	URL         OptURI    `json:"url"`
	Description OptString `json:"description"`
	TgChatIds   []int64   `json:"tgChatIds"`
}

// GetID returns the value of ID.
func (s *LinkUpdate) GetID() OptInt64 {
	return s.ID
}

// GetURL returns the value of URL.
func (s *LinkUpdate) GetURL() OptURI {
	return s.URL
}

// GetDescription returns the value of Description.
func (s *LinkUpdate) GetDescription() OptString {
	return s.Description
}

// GetTgChatIds returns the value of TgChatIds.
func (s *LinkUpdate) GetTgChatIds() []int64 {
	return s.TgChatIds
}

// SetID sets the value of ID.
func (s *LinkUpdate) SetID(val OptInt64) {
	s.ID = val
}

// SetURL sets the value of URL.
func (s *LinkUpdate) SetURL(val OptURI) {
	s.URL = val
}

// SetDescription sets the value of Description.
func (s *LinkUpdate) SetDescription(val OptString) {
	s.Description = val
}

// SetTgChatIds sets the value of TgChatIds.
func (s *LinkUpdate) SetTgChatIds(val []int64) {
	s.TgChatIds = val
}

// NewOptInt64 returns new OptInt64 with value set to v.
func NewOptInt64(v int64) OptInt64 {
	return OptInt64{
		Value: v,
		Set:   true,
	}
}

// OptInt64 is optional int64.
type OptInt64 struct {
	Value int64
	Set   bool
}

// IsSet returns true if OptInt64 was set.
func (o OptInt64) IsSet() bool { return o.Set }

// Reset unsets value.
func (o *OptInt64) Reset() {
	var v int64
	o.Value = v
	o.Set = false
}

// SetTo sets value to v.
func (o *OptInt64) SetTo(v int64) {
	o.Set = true
	o.Value = v
}

// Get returns value and boolean that denotes whether value was set.
func (o OptInt64) Get() (v int64, ok bool) {
	if !o.Set {
		return v, false
	}
	return o.Value, true
}

// Or returns value if set, or given parameter if does not.
func (o OptInt64) Or(d int64) int64 {
	if v, ok := o.Get(); ok {
		return v
	}
	return d
}

// NewOptString returns new OptString with value set to v.
func NewOptString(v string) OptString {
	return OptString{
		Value: v,
		Set:   true,
	}
}

// OptString is optional string.
type OptString struct {
	Value string
	Set   bool
}

// IsSet returns true if OptString was set.
func (o OptString) IsSet() bool { return o.Set }

// Reset unsets value.
func (o *OptString) Reset() {
	var v string
	o.Value = v
	o.Set = false
}

// SetTo sets value to v.
func (o *OptString) SetTo(v string) {
	o.Set = true
	o.Value = v
}

// Get returns value and boolean that denotes whether value was set.
func (o OptString) Get() (v string, ok bool) {
	if !o.Set {
		return v, false
	}
	return o.Value, true
}

// Or returns value if set, or given parameter if does not.
func (o OptString) Or(d string) string {
	if v, ok := o.Get(); ok {
		return v
	}
	return d
}

// NewOptURI returns new OptURI with value set to v.
func NewOptURI(v url.URL) OptURI {
	return OptURI{
		Value: v,
		Set:   true,
	}
}

// OptURI is optional url.URL.
type OptURI struct {
	Value url.URL
	Set   bool
}

// IsSet returns true if OptURI was set.
func (o OptURI) IsSet() bool { return o.Set }

// Reset unsets value.
func (o *OptURI) Reset() {
	var v url.URL
	o.Value = v
	o.Set = false
}

// SetTo sets value to v.
func (o *OptURI) SetTo(v url.URL) {
	o.Set = true
	o.Value = v
}

// Get returns value and boolean that denotes whether value was set.
func (o OptURI) Get() (v url.URL, ok bool) {
	if !o.Set {
		return v, false
	}
	return o.Value, true
}

// Or returns value if set, or given parameter if does not.
func (o OptURI) Or(d url.URL) url.URL {
	if v, ok := o.Get(); ok {
		return v
	}
	return d
}

// UpdatesPostOK is response for UpdatesPost operation.
type UpdatesPostOK struct{}

func (*UpdatesPostOK) updatesPostRes() {}
