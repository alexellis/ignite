package serializer

import (
	"fmt"
	"io/ioutil"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
)

// Serializer is an interface providing high-level decoding/encoding functionality
// for types registered in a *runtime.Scheme
type Serializer interface {
	// DecodeInto takes byte content and a target object to serialize the data into
	DecodeInto(content []byte, obj runtime.Object) error
	// DecodeFileInto takes a file path and a target object to serialize the data into
	DecodeFileInto(filePath string, obj runtime.Object) error

	// Decode takes byte content and returns the target object
	Decode(content []byte) (runtime.Object, error)
	// DecodeFile takes a file path and returns the target object
	DecodeFile(filePath string) (runtime.Object, error)

	// EncodeYAML encodes the specified object for a specific version to YAML bytes
	EncodeYAML(obj runtime.Object) ([]byte, error)
	// EncodeJSON encodes the specified object for a specific version to pretty JSON bytes
	EncodeJSON(obj runtime.Object) ([]byte, error)

	// Scheme provides access to the underlying runtime.Scheme
	Scheme() *runtime.Scheme
}

// NewSerializer constructs a new serializer based on a scheme, and optionally a codecfactory
func NewSerializer(scheme *runtime.Scheme, codecs *k8sserializer.CodecFactory) Serializer {
	if scheme == nil {
		panic("scheme must not be nil")
	}

	if codecs == nil {
		codecs = &k8sserializer.CodecFactory{}
		*codecs = k8sserializer.NewCodecFactory(scheme)
	}

	return &serializer{
		scheme: scheme,
		codecs: codecs,
		decoder: json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{
			Yaml:   true,
			Strict: true,
		}),
	}
}

// serializer implements the Serializer interface
type serializer struct {
	scheme  *runtime.Scheme
	codecs  *k8sserializer.CodecFactory
	decoder runtime.Decoder
}

// Scheme provides access to the underlying runtime.Scheme
func (s *serializer) Scheme() *runtime.Scheme {
	return s.scheme
}

// DecodeFileInto takes a file path and a target object to serialize the data into
func (s *serializer) DecodeFileInto(filePath string, obj runtime.Object) error {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	return s.DecodeInto(content, obj)
}

// DecodeInto takes byte content and a target object to serialize the data into
func (s *serializer) DecodeInto(content []byte, obj runtime.Object) error {
	return runtime.DecodeInto(s.decoder, content, obj)
}

// DecodeFile takes a file path and returns the target object
func (s *serializer) DecodeFile(filePath string) (runtime.Object, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return s.Decode(content)
}

// Decode takes byte content and returns the target object
func (s *serializer) Decode(content []byte) (runtime.Object, error) {
	return runtime.Decode(s.decoder, content)
}

// EncodeYAML encodes the specified object for a specific version to YAML bytes
func (s *serializer) EncodeYAML(obj runtime.Object) ([]byte, error) {
	return s.encode(obj, runtime.ContentTypeYAML, false)
}

// EncodeJSON encodes the specified object for a specific version to pretty JSON bytes
func (s *serializer) EncodeJSON(obj runtime.Object) ([]byte, error) {
	return s.encode(obj, runtime.ContentTypeJSON, true)
}

func (s *serializer) encode(obj runtime.Object, mediaType string, pretty bool) ([]byte, error) {
	info, ok := runtime.SerializerInfoForMediaType(s.codecs.SupportedMediaTypes(), mediaType)
	if !ok {
		return nil, fmt.Errorf("unable to locate encoder -- %q is not a supported media type", mediaType)
	}

	serializer := info.Serializer
	if pretty {
		serializer = info.PrettySerializer
	}

	gvk, err := s.externalGVKForObject(obj)
	if err != nil {
		return nil, err
	}

	encoder := s.codecs.EncoderForVersion(serializer, gvk.GroupVersion())
	return runtime.Encode(encoder, obj)
}

func (s *serializer) externalGVKForObject(cfg runtime.Object) (*schema.GroupVersionKind, error) {
	gvks, unversioned, err := s.scheme.ObjectKinds(cfg)
	if unversioned || err != nil || len(gvks) == 0 {
		return nil, fmt.Errorf("unversioned %t or err %v or invalid gvks %v", unversioned, err, gvks)
	}

	gvk := gvks[0]
	gvs := s.scheme.PrioritizedVersionsForGroup(gvk.Group)
	if len(gvs) < 1 {
		return nil, fmt.Errorf("expected some version to be registered for group %s", gvk.Group)
	}

	// Use the preferred (external) version
	gvk.Version = gvs[0].Version
	return &gvk, nil
}
