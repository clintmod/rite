package task_test

import (
	"os"
	"testing"

	"github.com/clintmod/rite/internal/filepathext"
	task "github.com/clintmod/rite/internal/task"
)

func TestInitDir(t *testing.T) {
	t.Parallel()

	const dir = "testdata/init"
	file := filepathext.SmartJoin(dir, "Ritefile.yml")

	_ = os.Remove(file)
	if _, err := os.Stat(file); err == nil {
		t.Errorf("Ritefile.yml should not exist")
	}

	if _, err := task.InitRitefile(dir); err != nil {
		t.Error(err)
	}

	if _, err := os.Stat(file); err != nil {
		t.Errorf("Ritefile.yml should exist")
	}

	_ = os.Remove(file)
}

func TestInitFile(t *testing.T) {
	t.Parallel()

	const dir = "testdata/init"
	file := filepathext.SmartJoin(dir, "Tasks.yml")

	_ = os.Remove(file)
	if _, err := os.Stat(file); err == nil {
		t.Errorf("Tasks.yml should not exist")
	}

	if _, err := task.InitRitefile(file); err != nil {
		t.Error(err)
	}

	if _, err := os.Stat(file); err != nil {
		t.Errorf("Tasks.yml should exist")
	}
	_ = os.Remove(file)
}
