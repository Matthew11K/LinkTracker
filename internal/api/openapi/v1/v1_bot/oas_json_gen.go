// Code generated by ogen, DO NOT EDIT.

package v1_bot

import (
	"github.com/go-faster/errors"
	"github.com/go-faster/jx"

	"github.com/ogen-go/ogen/json"
)

// Encode implements json.Marshaler.
func (s *ApiErrorResponse) Encode(e *jx.Encoder) {
	e.ObjStart()
	s.encodeFields(e)
	e.ObjEnd()
}

// encodeFields encodes fields.
func (s *ApiErrorResponse) encodeFields(e *jx.Encoder) {
	{
		if s.Description.Set {
			e.FieldStart("description")
			s.Description.Encode(e)
		}
	}
	{
		if s.Code.Set {
			e.FieldStart("code")
			s.Code.Encode(e)
		}
	}
	{
		if s.ExceptionName.Set {
			e.FieldStart("exceptionName")
			s.ExceptionName.Encode(e)
		}
	}
	{
		if s.ExceptionMessage.Set {
			e.FieldStart("exceptionMessage")
			s.ExceptionMessage.Encode(e)
		}
	}
	{
		if s.Stacktrace != nil {
			e.FieldStart("stacktrace")
			e.ArrStart()
			for _, elem := range s.Stacktrace {
				e.Str(elem)
			}
			e.ArrEnd()
		}
	}
}

var jsonFieldsNameOfApiErrorResponse = [5]string{
	0: "description",
	1: "code",
	2: "exceptionName",
	3: "exceptionMessage",
	4: "stacktrace",
}

// Decode decodes ApiErrorResponse from json.
func (s *ApiErrorResponse) Decode(d *jx.Decoder) error {
	if s == nil {
		return errors.New("invalid: unable to decode ApiErrorResponse to nil")
	}

	if err := d.ObjBytes(func(d *jx.Decoder, k []byte) error {
		switch string(k) {
		case "description":
			if err := func() error {
				s.Description.Reset()
				if err := s.Description.Decode(d); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return errors.Wrap(err, "decode field \"description\"")
			}
		case "code":
			if err := func() error {
				s.Code.Reset()
				if err := s.Code.Decode(d); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return errors.Wrap(err, "decode field \"code\"")
			}
		case "exceptionName":
			if err := func() error {
				s.ExceptionName.Reset()
				if err := s.ExceptionName.Decode(d); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return errors.Wrap(err, "decode field \"exceptionName\"")
			}
		case "exceptionMessage":
			if err := func() error {
				s.ExceptionMessage.Reset()
				if err := s.ExceptionMessage.Decode(d); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return errors.Wrap(err, "decode field \"exceptionMessage\"")
			}
		case "stacktrace":
			if err := func() error {
				s.Stacktrace = make([]string, 0)
				if err := d.Arr(func(d *jx.Decoder) error {
					var elem string
					v, err := d.Str()
					elem = string(v)
					if err != nil {
						return err
					}
					s.Stacktrace = append(s.Stacktrace, elem)
					return nil
				}); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return errors.Wrap(err, "decode field \"stacktrace\"")
			}
		default:
			return d.Skip()
		}
		return nil
	}); err != nil {
		return errors.Wrap(err, "decode ApiErrorResponse")
	}

	return nil
}

// MarshalJSON implements stdjson.Marshaler.
func (s *ApiErrorResponse) MarshalJSON() ([]byte, error) {
	e := jx.Encoder{}
	s.Encode(&e)
	return e.Bytes(), nil
}

// UnmarshalJSON implements stdjson.Unmarshaler.
func (s *ApiErrorResponse) UnmarshalJSON(data []byte) error {
	d := jx.DecodeBytes(data)
	return s.Decode(d)
}

// Encode implements json.Marshaler.
func (s *LinkUpdate) Encode(e *jx.Encoder) {
	e.ObjStart()
	s.encodeFields(e)
	e.ObjEnd()
}

// encodeFields encodes fields.
func (s *LinkUpdate) encodeFields(e *jx.Encoder) {
	{
		if s.ID.Set {
			e.FieldStart("id")
			s.ID.Encode(e)
		}
	}
	{
		if s.URL.Set {
			e.FieldStart("url")
			s.URL.Encode(e)
		}
	}
	{
		if s.Description.Set {
			e.FieldStart("description")
			s.Description.Encode(e)
		}
	}
	{
		if s.TgChatIds != nil {
			e.FieldStart("tgChatIds")
			e.ArrStart()
			for _, elem := range s.TgChatIds {
				e.Int64(elem)
			}
			e.ArrEnd()
		}
	}
}

var jsonFieldsNameOfLinkUpdate = [4]string{
	0: "id",
	1: "url",
	2: "description",
	3: "tgChatIds",
}

// Decode decodes LinkUpdate from json.
func (s *LinkUpdate) Decode(d *jx.Decoder) error {
	if s == nil {
		return errors.New("invalid: unable to decode LinkUpdate to nil")
	}

	if err := d.ObjBytes(func(d *jx.Decoder, k []byte) error {
		switch string(k) {
		case "id":
			if err := func() error {
				s.ID.Reset()
				if err := s.ID.Decode(d); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return errors.Wrap(err, "decode field \"id\"")
			}
		case "url":
			if err := func() error {
				s.URL.Reset()
				if err := s.URL.Decode(d); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return errors.Wrap(err, "decode field \"url\"")
			}
		case "description":
			if err := func() error {
				s.Description.Reset()
				if err := s.Description.Decode(d); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return errors.Wrap(err, "decode field \"description\"")
			}
		case "tgChatIds":
			if err := func() error {
				s.TgChatIds = make([]int64, 0)
				if err := d.Arr(func(d *jx.Decoder) error {
					var elem int64
					v, err := d.Int64()
					elem = int64(v)
					if err != nil {
						return err
					}
					s.TgChatIds = append(s.TgChatIds, elem)
					return nil
				}); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return errors.Wrap(err, "decode field \"tgChatIds\"")
			}
		default:
			return d.Skip()
		}
		return nil
	}); err != nil {
		return errors.Wrap(err, "decode LinkUpdate")
	}

	return nil
}

// MarshalJSON implements stdjson.Marshaler.
func (s *LinkUpdate) MarshalJSON() ([]byte, error) {
	e := jx.Encoder{}
	s.Encode(&e)
	return e.Bytes(), nil
}

// UnmarshalJSON implements stdjson.Unmarshaler.
func (s *LinkUpdate) UnmarshalJSON(data []byte) error {
	d := jx.DecodeBytes(data)
	return s.Decode(d)
}

// Encode encodes int64 as json.
func (o OptInt64) Encode(e *jx.Encoder) {
	if !o.Set {
		return
	}
	e.Int64(int64(o.Value))
}

// Decode decodes int64 from json.
func (o *OptInt64) Decode(d *jx.Decoder) error {
	if o == nil {
		return errors.New("invalid: unable to decode OptInt64 to nil")
	}
	o.Set = true
	v, err := d.Int64()
	if err != nil {
		return err
	}
	o.Value = int64(v)
	return nil
}

// MarshalJSON implements stdjson.Marshaler.
func (s OptInt64) MarshalJSON() ([]byte, error) {
	e := jx.Encoder{}
	s.Encode(&e)
	return e.Bytes(), nil
}

// UnmarshalJSON implements stdjson.Unmarshaler.
func (s *OptInt64) UnmarshalJSON(data []byte) error {
	d := jx.DecodeBytes(data)
	return s.Decode(d)
}

// Encode encodes string as json.
func (o OptString) Encode(e *jx.Encoder) {
	if !o.Set {
		return
	}
	e.Str(string(o.Value))
}

// Decode decodes string from json.
func (o *OptString) Decode(d *jx.Decoder) error {
	if o == nil {
		return errors.New("invalid: unable to decode OptString to nil")
	}
	o.Set = true
	v, err := d.Str()
	if err != nil {
		return err
	}
	o.Value = string(v)
	return nil
}

// MarshalJSON implements stdjson.Marshaler.
func (s OptString) MarshalJSON() ([]byte, error) {
	e := jx.Encoder{}
	s.Encode(&e)
	return e.Bytes(), nil
}

// UnmarshalJSON implements stdjson.Unmarshaler.
func (s *OptString) UnmarshalJSON(data []byte) error {
	d := jx.DecodeBytes(data)
	return s.Decode(d)
}

// Encode encodes url.URL as json.
func (o OptURI) Encode(e *jx.Encoder) {
	if !o.Set {
		return
	}
	json.EncodeURI(e, o.Value)
}

// Decode decodes url.URL from json.
func (o *OptURI) Decode(d *jx.Decoder) error {
	if o == nil {
		return errors.New("invalid: unable to decode OptURI to nil")
	}
	o.Set = true
	v, err := json.DecodeURI(d)
	if err != nil {
		return err
	}
	o.Value = v
	return nil
}

// MarshalJSON implements stdjson.Marshaler.
func (s OptURI) MarshalJSON() ([]byte, error) {
	e := jx.Encoder{}
	s.Encode(&e)
	return e.Bytes(), nil
}

// UnmarshalJSON implements stdjson.Unmarshaler.
func (s *OptURI) UnmarshalJSON(data []byte) error {
	d := jx.DecodeBytes(data)
	return s.Decode(d)
}
