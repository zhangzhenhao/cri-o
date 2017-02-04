package docker

import (
	"testing"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sha256digestHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	sha256digest    = "@sha256:" + sha256digestHex
)

func TestTransportName(t *testing.T) {
	assert.Equal(t, "docker", Transport.Name())
}

func TestTransportParseReference(t *testing.T) {
	testParseReference(t, Transport.ParseReference)
}

func TestTransportValidatePolicyConfigurationScope(t *testing.T) {
	for _, scope := range []string{
		"docker.io/library/busybox" + sha256digest,
		"docker.io/library/busybox:notlatest",
		"docker.io/library/busybox",
		"docker.io/library",
		"docker.io",
	} {
		err := Transport.ValidatePolicyConfigurationScope(scope)
		assert.NoError(t, err, scope)
	}
}

func TestParseReference(t *testing.T) {
	testParseReference(t, ParseReference)
}

// testParseReference is a test shared for Transport.ParseReference and ParseReference.
func testParseReference(t *testing.T, fn func(string) (types.ImageReference, error)) {
	for _, c := range []struct{ input, expected string }{
		{"busybox", ""},                                        // Missing // prefix
		{"//busybox:notlatest", "busybox:notlatest"},           // Explicit tag
		{"//busybox" + sha256digest, "busybox" + sha256digest}, // Explicit digest
		{"//busybox", "busybox:latest"},                        // Default tag
		// A github.com/distribution/reference value can have a tag and a digest at the same time!
		// github.com/docker/reference handles that by dropping the tag. That is not obviously the
		// right thing to do, but it is at least reasonable, so test that we keep behaving reasonably.
		// This test case should not be construed to make this an API promise.
		// FIXME? Instead work extra hard to reject such input?
		{"//busybox:latest" + sha256digest, "busybox" + sha256digest}, // Both tag and digest
		{"//docker.io/library/busybox:latest", "busybox:latest"},      // All implied values explicitly specified
		{"//UPPERCASEISINVALID", ""},                                  // Invalid input
	} {
		ref, err := fn(c.input)
		if c.expected == "" {
			assert.Error(t, err, c.input)
		} else {
			require.NoError(t, err, c.input)
			dockerRef, ok := ref.(dockerReference)
			require.True(t, ok, c.input)
			assert.Equal(t, c.expected, dockerRef.ref.String(), c.input)
		}
	}
}

// refWithTagAndDigest is a reference.NamedTagged and reference.Canonical at the same time.
type refWithTagAndDigest struct{ reference.Canonical }

func (ref refWithTagAndDigest) Tag() string {
	return "notLatest"
}

// A common list of reference formats to test for the various ImageReference methods.
var validReferenceTestCases = []struct{ input, dockerRef, stringWithinTransport string }{
	{"busybox:notlatest", "busybox:notlatest", "//busybox:notlatest"},                // Explicit tag
	{"busybox" + sha256digest, "busybox" + sha256digest, "//busybox" + sha256digest}, // Explicit digest
	{"docker.io/library/busybox:latest", "busybox:latest", "//busybox:latest"},       // All implied values explicitly specified
	{"example.com/ns/foo:bar", "example.com/ns/foo:bar", "//example.com/ns/foo:bar"}, // All values explicitly specified
}

func TestNewReference(t *testing.T) {
	for _, c := range validReferenceTestCases {
		parsed, err := reference.ParseNamed(c.input)
		require.NoError(t, err)
		ref, err := NewReference(parsed)
		require.NoError(t, err, c.input)
		dockerRef, ok := ref.(dockerReference)
		require.True(t, ok, c.input)
		assert.Equal(t, c.dockerRef, dockerRef.ref.String(), c.input)
	}

	// Neither a tag nor digest
	parsed, err := reference.ParseNamed("busybox")
	require.NoError(t, err)
	_, err = NewReference(parsed)
	assert.Error(t, err)

	// A github.com/distribution/reference value can have a tag and a digest at the same time!
	parsed, err = reference.ParseNamed("busybox" + sha256digest)
	require.NoError(t, err)
	refDigested, ok := parsed.(reference.Canonical)
	require.True(t, ok)
	tagDigestRef := refWithTagAndDigest{refDigested}
	_, err = NewReference(tagDigestRef)
	assert.Error(t, err)
}

func TestReferenceTransport(t *testing.T) {
	ref, err := ParseReference("//busybox")
	require.NoError(t, err)
	assert.Equal(t, Transport, ref.Transport())
}

func TestReferenceStringWithinTransport(t *testing.T) {
	for _, c := range validReferenceTestCases {
		ref, err := ParseReference("//" + c.input)
		require.NoError(t, err, c.input)
		stringRef := ref.StringWithinTransport()
		assert.Equal(t, c.stringWithinTransport, stringRef, c.input)
		// Do one more round to verify that the output can be parsed, to an equal value.
		ref2, err := Transport.ParseReference(stringRef)
		require.NoError(t, err, c.input)
		stringRef2 := ref2.StringWithinTransport()
		assert.Equal(t, stringRef, stringRef2, c.input)
	}
}

func TestReferenceDockerReference(t *testing.T) {
	for _, c := range validReferenceTestCases {
		ref, err := ParseReference("//" + c.input)
		require.NoError(t, err, c.input)
		dockerRef := ref.DockerReference()
		require.NotNil(t, dockerRef, c.input)
		assert.Equal(t, c.dockerRef, dockerRef.String(), c.input)
	}
}

func TestReferencePolicyConfigurationIdentity(t *testing.T) {
	// Just a smoke test, the substance is tested in policyconfiguration.TestDockerReference.
	ref, err := ParseReference("//busybox")
	require.NoError(t, err)
	assert.Equal(t, "docker.io/library/busybox:latest", ref.PolicyConfigurationIdentity())
}

func TestReferencePolicyConfigurationNamespaces(t *testing.T) {
	// Just a smoke test, the substance is tested in policyconfiguration.TestDockerReference.
	ref, err := ParseReference("//busybox")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"docker.io/library/busybox",
		"docker.io/library",
		"docker.io",
	}, ref.PolicyConfigurationNamespaces())
}

func TestReferenceNewImage(t *testing.T) {
	ref, err := ParseReference("//busybox")
	require.NoError(t, err)
	img, err := ref.NewImage(&types.SystemContext{RegistriesDirPath: "/this/doesnt/exist"})
	assert.NoError(t, err)
	defer img.Close()
}

func TestReferenceNewImageSource(t *testing.T) {
	ref, err := ParseReference("//busybox")
	require.NoError(t, err)
	src, err := ref.NewImageSource(&types.SystemContext{RegistriesDirPath: "/this/doesnt/exist"}, nil)
	assert.NoError(t, err)
	defer src.Close()
}

func TestReferenceNewImageDestination(t *testing.T) {
	ref, err := ParseReference("//busybox")
	require.NoError(t, err)
	dest, err := ref.NewImageDestination(&types.SystemContext{RegistriesDirPath: "/this/doesnt/exist"})
	assert.NoError(t, err)
	defer dest.Close()
}

func TestReferenceTagOrDigest(t *testing.T) {
	for input, expected := range map[string]string{
		"//busybox:notlatest":      "notlatest",
		"//busybox" + sha256digest: "sha256:" + sha256digestHex,
	} {
		ref, err := ParseReference(input)
		require.NoError(t, err, input)
		dockerRef, ok := ref.(dockerReference)
		require.True(t, ok, input)
		tod, err := dockerRef.tagOrDigest()
		require.NoError(t, err, input)
		assert.Equal(t, expected, tod, input)
	}

	// Invalid input
	ref, err := reference.ParseNamed("busybox")
	require.NoError(t, err)
	dockerRef := dockerReference{ref: ref}
	_, err = dockerRef.tagOrDigest()
	assert.Error(t, err)
}