package gontainer

import (
	"context"
	"fmt"
	"testing"
)

// TestFactoryLoad tests factory loading.
func TestFactoryLoad(t *testing.T) {
	fun := func(a, b, c string) (int, bool, error) {
		return 1, true, nil
	}

	ctx := context.Background()
	opts := WithMetadata("test", "value")
	factory := NewFactory(fun, opts)

	equal(t, factory.load(ctx), nil)
	equal(t, factory.factoryFunc == nil, false)
	equal(t, factory.factoryLoaded, true)
	equal(t, factory.factorySpawned, false)
	equal(t, factory.factoryCtx != ctx, true)
	equal(t, factory.factoryCtx != nil, true)
	equal(t, factory.ctxCancel != nil, true)
	equal(t, factory.factoryType.String(), "func(string, string, string) (int, bool, error)")
	equal(t, factory.factoryValue.String(), "<func(string, string, string) (int, bool, error) Value>")
	equal(t, fmt.Sprint(factory.factoryInTypes), "[string string string]")
	equal(t, fmt.Sprint(factory.factoryOutTypes), "[int bool]")
	equal(t, factory.factoryOutError, true)
	equal(t, len(factory.factoryOutValues), 0)
	equal(t, factory.factoryMetadata["test"], "value")
}

// TestFactoryInfo tests factories info.
func TestFactoryInfo(t *testing.T) {
	type localType struct{}

	localFunc := func(globalType) string {
		return "string"
	}

	tests := []struct {
		name  string
		arg1  *Factory
		want1 string
		want2 string
	}{
		{
			name:  "ServiceLocalType",
			arg1:  NewService(localType{}),
			want1: "Service[gontainer.localType]",
			want2: "github.com/NVIDIA/gontainer",
		},
		{
			name:  "ServiceGlobalType",
			arg1:  NewService(globalType{}),
			want1: "Service[gontainer.globalType]",
			want2: "github.com/NVIDIA/gontainer",
		},
		{
			name:  "FactoryLocalFunc",
			arg1:  NewFactory(localFunc),
			want1: "Factory[func(gontainer.globalType) string]",
			want2: "github.com/NVIDIA/gontainer",
		},
		{
			name:  "FactoryGlobalFunc",
			arg1:  NewFactory(globalFunc),
			want1: "Factory[func(string)]",
			want2: "github.com/NVIDIA/gontainer",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got1 := tt.arg1.Name()
			if got1 != tt.want1 {
				t.Errorf("Factory.Name() got = %v, want %v", got1, tt.want1)
			}
			got2 := tt.arg1.Source()
			if got2 != tt.want2 {
				t.Errorf("Factory.Source() got = %v, want %v", got2, tt.want2)
			}
		})
	}
}

type globalType struct{}

func globalFunc(string) {}

// TestSplitFuncName tests splitting of function name.
func TestSplitFuncName(t *testing.T) {
	tests := []struct {
		name  string
		arg   string
		want1 string
		want2 string
	}{{
		name:  "SplitPublicPackage",
		arg:   "github.com/NVIDIA/gontainer/app.WithApp.func1",
		want1: "github.com/NVIDIA/gontainer/app",
		want2: "WithApp.func1",
	}, {
		name:  "SplitMainPackage",
		arg:   "main.main.func1",
		want1: "main",
		want2: "main.func1",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got1, got2 := splitFuncName(tt.arg)
			if got1 != tt.want1 {
				t.Errorf("splitFuncName() got1 = %v, want %v", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Errorf("splitFuncName() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}
