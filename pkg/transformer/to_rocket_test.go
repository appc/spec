package transformer

import (
	"strings"
	"testing"

	"github.com/docker/docker/builder/parser"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type TransformerSuite struct{}

var _ = Suite(&TransformerSuite{})

func (s *TransformerSuite) GetNode(dockerfile string) *parser.Node {
	o, err := parser.Parse(strings.NewReader(dockerfile))
	if err != nil {
		panic(err)
	}

	return o
}

func (s *TransformerSuite) TestNewTransformer(c *C) {
	t, err := NewToRocket("foo", "1.0.0", "linux", "amd64")

	c.Assert(err, IsNil)
	c.Assert(t.manifest.Name.String(), Equals, "foo")
	c.Assert(t.manifest.Version.String(), Equals, "1.0.0")
	c.Assert(t.manifest.OS.String(), Equals, "linux")
	c.Assert(t.manifest.Arch.String(), Equals, "amd64")
}

func (s *TransformerSuite) TestTransformer_processCMDNodeJSON(c *C) {
	t, _ := NewToRocket("foo", "1.0.0", "linux", "amd64")
	t.Process(s.GetNode("CMD [\"foo\", \"bar\"]"))

	c.Assert(t.manifest.Exec, HasLen, 1)
	c.Assert(t.manifest.Exec[0], Equals, "foo bar")
}

func (s *TransformerSuite) TestTransformer_processCMDNodePlain(c *C) {
	t, _ := NewToRocket("foo", "1.0.0", "linux", "amd64")
	t.Process(s.GetNode("CMD foo bar"))

	c.Assert(t.manifest.Exec, HasLen, 1)
	c.Assert(t.manifest.Exec[0], Equals, "foo bar")
}

func (s *TransformerSuite) TestTransformer_processCMDAndEntrypointNodePlain(c *C) {
	t, _ := NewToRocket("foo", "1.0.0", "linux", "amd64")
	t.Process(s.GetNode("CMD -a\nENTRYPOINT ls"))

	c.Assert(t.manifest.Exec, HasLen, 1)
	c.Assert(t.manifest.Exec[0], Equals, "ls -a")
}

func (s *TransformerSuite) TestTransformer_processVolumeNode(c *C) {
	t, _ := NewToRocket("foo", "1.0.0", "linux", "amd64")
	t.Process(s.GetNode("VOLUME [\"/foo/bar\", \"qux\"]"))

	c.Assert(t.manifest.MountPoints, HasLen, 2)
	c.Assert(t.manifest.MountPoints[0].Name.String(), Equals, "bar")
	c.Assert(t.manifest.MountPoints[0].Path, Equals, "/foo/bar")
}

func (s *TransformerSuite) TestTransformer_processExposeNode(c *C) {
	t, _ := NewToRocket("foo", "1.0.0", "linux", "amd64")
	t.Process(s.GetNode("EXPOSE 80\nEXPOSE 22/udp"))

	c.Assert(t.manifest.Ports, HasLen, 2)
	c.Assert(t.manifest.Ports[0].Name.String(), Equals, "80")
	c.Assert(t.manifest.Ports[0].Port, Equals, uint(80))
	c.Assert(t.manifest.Ports[0].Protocol, Equals, "tcp")
	c.Assert(t.manifest.Ports[1].Name.String(), Equals, "22")
	c.Assert(t.manifest.Ports[1].Port, Equals, uint(22))
	c.Assert(t.manifest.Ports[1].Protocol, Equals, "udp")
}

func (s *TransformerSuite) TestTransformer_processEnvNode(c *C) {
	t, _ := NewToRocket("foo", "1.0.0", "linux", "amd64")
	t.Process(s.GetNode("ENV FOO bar\nENV QUX baz"))

	c.Assert(t.manifest.Environment, HasLen, 2)
	c.Assert(t.manifest.Environment["FOO"], Equals, "bar")
	c.Assert(t.manifest.Environment["QUX"], Equals, "baz")
}

func (s *TransformerSuite) TestTransformer_processAddNode(c *C) {
	t, _ := NewToRocket("foo", "1.0.0", "linux", "amd64")
	t.Process(s.GetNode("CMD foo\nADD . example"))

	_, err := t.SaveToFile("/tmp/rocket_add")
	c.Assert(err, IsNil)
}

func (s *TransformerSuite) TestTransformer_processCopyNode(c *C) {
	t, _ := NewToRocket("foo", "1.0.0", "linux", "amd64")
	t.Process(s.GetNode("CMD foo\nCOPY . example"))

	_, err := t.SaveToFile("/tmp/rocket_copy")
	c.Assert(err, IsNil)
}
