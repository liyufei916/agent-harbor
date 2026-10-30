package dsh

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestConversationBuildUsesFixedDriverAndFramedInput(t *testing.T) {
	k := fixtureKit(t)
	k.conversation = true
	input := validInput()
	launch, err := k.Build(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !launch.Interactive || launch.EncodeInput == nil || len(launch.ConfigFiles) != 4 {
		t.Fatalf("%+v", launch)
	}
	encoded, err := launch.EncodeInput([]byte("round\n\"two\""))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(encoded), "\n") != 1 {
		t.Fatal("正文注入额外协议消息")
	}
	var message struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(encoded, &message); err != nil || message.Text != "round\n\"two\"" {
		t.Fatal("输入编码错误", err)
	}
	source := string(launch.ConfigFiles[3].Content)
	if strings.Contains(source, "__AGENT_MODULE__") || strings.Contains(source, input.Task) {
		t.Fatal("驱动未解析或混入任务")
	}
	for _, bad := range [][]byte{nil, []byte(" "), {0xff}, {0}, []byte(strings.Repeat("a", 32769))} {
		if _, err := launch.EncodeInput(bad); err == nil {
			t.Fatal("接受非法输入")
		}
	}
}

func TestConversationProfileUsesGuestPathUnderDocker(t *testing.T) {
	t.Setenv("HARBOR_RUNTIME", "docker")
	k := fixtureKit(t)
	k.conversation = true
	input := validInput()
	input.PrivateHome = "/Users/he.s/Library/Application Support/AgentHarbor/sessions/x/home"
	launch, err := k.Build(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	var patches []struct {
		Insert []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"insert"`
	}
	found := false
	for _, file := range launch.ConfigFiles {
		if file.RelativePath != "dsh/profiles/harbor/cordis.patch.yml" {
			continue
		}
		if err := json.Unmarshal(file.Content, &patches); err != nil {
			t.Fatal(err)
		}
		for _, row := range patches[0].Insert {
			if row.ID != "harbor-conversation" {
				continue
			}
			found = true
			if row.Name != "/home/harbor/harbor-conversation.mjs" {
				t.Fatalf("Docker guest 插件路径错误: %q", row.Name)
			}
			if strings.Contains(row.Name, "Application Support") || strings.Contains(row.Name, input.PrivateHome) {
				t.Fatalf("仍含宿主机路径: %q", row.Name)
			}
		}
	}
	if !found {
		t.Fatal("缺少 harbor-conversation 插件")
	}
}
