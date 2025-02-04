package codec

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/schema"
)

type Codec interface {
	// Decode parses the request body into the specified value.
	// Example:
	//	type User struct {
	//		Name string `json:"name"`
	//		Age  int    `json:"age"`
	//	}
	//	var user User
	//	err := codec.decode(req, &user)
	//	if err != nil {
	//		// Handle error
	//	}
	Decode(req *http.Request, val any) error

	// DecodePath decodes the `name` parameter from the request path.
	// The `val` type must be a pointer to a string or a number.
	// Example:
	// For a request: /api/v1/users/{id}
	//	var id int
	//	err := codec.decodePath(req, "id", &id)
	//	if err != nil {
	//		// Handle error
	//	}
	//	fmt.Println("Decoded ID:", id)
	DecodePath(req *http.Request, name string, val any) error

	// DecodeQuery parses the query parameters from the URL into the specified value.
	// Example:
	//	type Filter struct {
	//		Page int    `query:"page"`
	//		Sort string `query:"sort"`
	//	}
	//	var filter Filter
	//	err := codec.DecodeQuery(req, &filter)
	//	if err != nil {
	//		// Handle error
	//	}
	//	fmt.Println("Filter:", filter)
	DecodeQuery(*http.Request, any) error

	// DecodeHeader parses the query parameters from the URL into the specified value.
	// Example:
	//	type ApiToken struct {
	//		Key int    `header:"X-API-TOKEN"`
	//	}
	//	err := codec.DecodeHeader(req, &filter)
	//	if err != nil {
	//		// Handle error
	//	}
	//	fmt.Println("Filter:", filter)
	DecodeHeader(*http.Request, any) error

	// Encode serializes the specified value and writes it to the response body.
	// Example:
	//	data := map[string]string{"status": "success"}
	//	err := codec.encode(w, data)
	//	if err != nil {
	//		// Handle error
	//	}
	Encode(http.ResponseWriter, any) error

	// EncodeError serializes the error and writes it to the response body.
	// Example:
	//	err := errors.New("something went wrong")
	//	codec.encodeError(w, err)
	EncodeError(http.ResponseWriter, error) error
}

type codec struct {
	decode       func(req *http.Request, val any) error
	decodePath   func(req *http.Request, name string, val any) error
	decodeQuery  func(*http.Request, any) error
	decodeHeader func(*http.Request, any) error
	encode       func(http.ResponseWriter, any) error
	encodeError  func(http.ResponseWriter, error) error
}

func (c *codec) Decode(req *http.Request, val any) error {
	return c.decode(req, val)
}

func (c *codec) DecodePath(req *http.Request, name string, val any) error {
	return c.decodePath(req, name, val)
}

func (c *codec) DecodeQuery(request *http.Request, a any) error {
	return c.decodeQuery(request, a)
}

func (c *codec) DecodeHeader(request *http.Request, a any) error {
	return c.decodeHeader(request, a)
}

func (c *codec) Encode(writer http.ResponseWriter, a any) error {
	return c.encode(writer, a)
}

func (c *codec) EncodeError(writer http.ResponseWriter, err error) error {
	return c.encodeError(writer, err)
}

func WithDecode(decodeFunc func(*http.Request, any) error) func(c *codec) {
	return func(c *codec) {
		c.decode = decodeFunc
	}
}

func WithPathDecode(f func(req *http.Request, name string, val any) error) func(c *codec) {
	return func(c *codec) {
		c.decodePath = f
	}
}

func WithQueryDecode(f func(*http.Request, any) error) func(c *codec) {
	return func(c *codec) {
		c.decodeQuery = f
	}
}

func WithEncode(f func(http.ResponseWriter, any) error) func(c *codec) {
	return func(c *codec) {
		c.encode = f
	}
}

func EncodeError(f func(http.ResponseWriter, error) error) func(c *codec) {
	return func(c *codec) {
		c.encodeError = f
	}
}

type CodeCOption func(c *codec)

func New(opts ...CodeCOption) Codec {

	c := defaultCodec()
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func defaultCodec() *codec {
	return &codec{
		decode:       defaultDecode,
		decodePath:   defaultDecodePath,
		decodeQuery:  defaultDecodeQuery,
		decodeHeader: defaultDecodeHeader,
		encode:       defaultEncode,
		encodeError:  defaultEncodeError,
	}
}

func defaultDecode(req *http.Request, val any) error {
	return json.NewDecoder(req.Body).Decode(val)
}

func defaultDecodePath(req *http.Request, name string, val any) error {
	return nil
}

var headerDecoder = schema.NewDecoder()
var queryDecoder = schema.NewDecoder()

func init() {
	for _, decoder := range []*schema.Decoder{headerDecoder, queryDecoder} {
		decoder.IgnoreUnknownKeys(true)
		decoder.ZeroEmpty(true)
		decoder.SetAliasTag("header")
	}
}

func defaultDecodeQuery(req *http.Request, val any) error {
	return queryDecoder.Decode(val, req.URL.Query())
}

func defaultDecodeHeader(req *http.Request, val any) error {
	return headerDecoder.Decode(val, req.Header)
}

func defaultEncode(w http.ResponseWriter, val any) error {
	return json.NewEncoder(w).Encode(val)
}

func defaultEncodeError(w http.ResponseWriter, err error) error {

	w.WriteHeader(http.StatusInternalServerError)
	if _, err := w.Write([]byte(err.Error())); err != nil {
		return err
	}
	return nil
}
