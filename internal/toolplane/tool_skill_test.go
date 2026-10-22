package toolplane

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestToolSkillListAndRead(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	if err := runToolSkillList(cmd, nil); err != nil {
		t.Fatalf("runToolSkillList() error = %v", err)
	}
	var list ToolResponse
	if err := json.Unmarshal(out.Bytes(), &list); err != nil {
		t.Fatalf("decode skill list: %v\n%s", err, out.String())
	}
	if !list.OK || list.Section != "skill" || list.Count == 0 {
		t.Fatalf("unexpected skill list: %#v", list)
	}

	out.Reset()
	if err := runToolSkillRead(cmd, []string{"goal-agent-core"}); err != nil {
		t.Fatalf("runToolSkillRead() error = %v", err)
	}
	var read ToolResponse
	if err := json.Unmarshal(out.Bytes(), &read); err != nil {
		t.Fatalf("decode skill read: %v\n%s", err, out.String())
	}
	data, _ := json.Marshal(read.Results)
	if !strings.Contains(string(data), "Goal Agent Core") {
		t.Fatalf("skill read did not return the core skill body: %s", data)
	}
}

func TestToolSkillReadRejectsTraversal(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	if err := runToolSkillRead(cmd, []string{"../bad"}); err == nil {
		t.Fatal("expected invalid skill name to fail")
	}
}
