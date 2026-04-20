/*
 * SPDX-FileCopyrightText: Copyright (c) 2003 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package gontainer

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"testing"
)

// Synthetic types used to produce deterministic factory output types in rendered error traces.
type testFmtRootA struct{}
type testFmtRootB struct{}
type testFmtMid struct{}
type testFmtLeaf struct{}

// testFmtTypedErr is a custom user error type used to verify errors.As walking down the resolve chain.
type testFmtTypedErr struct {
	msg string
}

// Error returns the error message.
func (e *testFmtTypedErr) Error() string {
	return e.msg
}

// TestErrorFormatSingleFactoryOneLineCause checks a one-line cause with a two-frame Traceback.
func TestErrorFormatSingleFactoryOneLineCause(t *testing.T) {
	err := Run(
		NewFactory(func() (*testFmtLeaf, error) {
			return nil, errors.New("boom")
		}),
		NewEntrypoint(func(*testFmtLeaf) {}),
	)

	equal(t, err != nil, true)
	equal(t, normalizeSourceLines(err.Error()), ""+
		"boom"+
		"\n"+
		"\nTraceback:"+
		"\n  Factory for *gontainer.testFmtLeaf"+
		"\n  Entrypoint")
	equal(t, errors.Is(err, ErrFactoryReturnedError), true)
}

// TestErrorFormatDeepChainOneLineCause checks 4 frames with innermost first, outermost last.
func TestErrorFormatDeepChainOneLineCause(t *testing.T) {
	err := Run(
		NewFactory(func() (*testFmtLeaf, error) {
			return nil, errors.New("leaf boom")
		}),
		NewFactory(func(*testFmtLeaf) *testFmtMid {
			return &testFmtMid{}
		}),
		NewFactory(func(*testFmtMid) *testFmtRootA {
			return &testFmtRootA{}
		}),
		NewEntrypoint(func(*testFmtRootA) {}),
	)

	equal(t, err != nil, true)
	equal(t, normalizeSourceLines(err.Error()), ""+
		"leaf boom"+
		"\n"+
		"\nTraceback:"+
		"\n  Factory for *gontainer.testFmtLeaf"+
		"\n  Factory for *gontainer.testFmtMid"+
		"\n  Factory for *gontainer.testFmtRootA"+
		"\n  Entrypoint")
	equal(t, errors.Is(err, ErrFactoryReturnedError), true)
}

// TestErrorFormatDeepChainJoinCause checks that an errors.Join cause renders each item as a "- " bullet.
func TestErrorFormatDeepChainJoinCause(t *testing.T) {
	joined := errors.Join(
		errors.New("FIELD_A: 'required' rule failed"),
		errors.New("FIELD_B: 'required' rule failed"),
	)

	err := Run(
		NewFactory(func() (*testFmtLeaf, error) {
			return nil, joined
		}),
		NewFactory(func(*testFmtLeaf) *testFmtMid {
			return &testFmtMid{}
		}),
		NewEntrypoint(func(*testFmtMid) {}),
	)

	equal(t, err != nil, true)
	equal(t, normalizeSourceLines(err.Error()), ""+
		"- FIELD_A: 'required' rule failed"+
		"\n- FIELD_B: 'required' rule failed"+
		"\n"+
		"\nTraceback:"+
		"\n  Factory for *gontainer.testFmtLeaf"+
		"\n  Factory for *gontainer.testFmtMid"+
		"\n  Entrypoint")
	equal(t, errors.Is(err, ErrFactoryReturnedError), true)
}

// TestErrorFormatDeepChainWrapperOverJoin checks that a wrapper adds a header line above bullets from an inner Join.
func TestErrorFormatDeepChainWrapperOverJoin(t *testing.T) {
	joined := errors.Join(
		errors.New("URL: 'required' rule failed"),
		errors.New("TOKEN: 'required' rule failed"),
	)
	wrapped := fmt.Errorf("failed to load GitLab client config: %w", joined)

	err := Run(
		NewFactory(func() (*testFmtLeaf, error) {
			return nil, wrapped
		}),
		NewFactory(func(*testFmtLeaf) *testFmtMid {
			return &testFmtMid{}
		}),
		NewEntrypoint(func(*testFmtMid) {}),
	)

	equal(t, err != nil, true)
	equal(t, normalizeSourceLines(err.Error()), ""+
		"failed to load GitLab client config:"+
		"\n- URL: 'required' rule failed"+
		"\n- TOKEN: 'required' rule failed"+
		"\n"+
		"\nTraceback:"+
		"\n  Factory for *gontainer.testFmtLeaf"+
		"\n  Factory for *gontainer.testFmtMid"+
		"\n  Entrypoint")
	equal(t, errors.Is(err, ErrFactoryReturnedError), true)
}

// TestErrorFormatDependencyNotResolvedTail checks the "dependency not resolved: T" headline followed by a Traceback.
func TestErrorFormatDependencyNotResolvedTail(t *testing.T) {
	r := &registry{}
	equal(t, NewFactory(func(*testFmtLeaf) *testFmtMid {
		return &testFmtMid{}
	}).apply(r), nil)
	equal(t, NewEntrypoint(func(*testFmtMid) {}).apply(r), nil)

	err := r.invokeEntrypoints()

	equal(t, err != nil, true)
	equal(t, normalizeSourceLines(err.Error()), ""+
		"dependency not resolved: *gontainer.testFmtLeaf"+
		"\n"+
		"\nTraceback:"+
		"\n  Factory for *gontainer.testFmtMid"+
		"\n  Entrypoint")
	equal(t, errors.Is(err, ErrDependencyNotResolved), true)
}

// TestErrorFormatCircularDependencyTail checks that each factory in the cycle reports its own "circular dependency" block.
func TestErrorFormatCircularDependencyTail(t *testing.T) {
	err := Run(
		NewFactory(func(*testFmtMid) *testFmtRootA { return &testFmtRootA{} }),
		NewFactory(func(*testFmtRootA) *testFmtMid { return &testFmtMid{} }),
		NewEntrypoint(func(*testFmtRootA) {}),
	)

	equal(t, err != nil, true)
	equal(t, errors.Is(err, ErrCircularDependency), true)

	unwrap, ok := err.(interface{ Unwrap() []error })
	equal(t, ok, true)
	errs := unwrap.Unwrap()
	equal(t, len(errs), 2)
	equal(t, normalizeSourceLines(errs[0].Error()), ""+
		"circular dependency"+
		"\n"+
		"\nTraceback:"+
		"\n  Factory for *gontainer.testFmtRootA")
	equal(t, normalizeSourceLines(errs[1].Error()), ""+
		"circular dependency"+
		"\n"+
		"\nTraceback:"+
		"\n  Factory for *gontainer.testFmtMid")
}

// TestErrorFormatMultipleTopLevelChains checks that two independent entrypoint failures are separated by a single blank line.
func TestErrorFormatMultipleTopLevelChains(t *testing.T) {
	err := Run(
		NewFactory(func() (*testFmtLeaf, error) {
			return nil, errors.New("leaf error")
		}),
		NewEntrypoint(func(*testFmtLeaf) error { return nil }),
		NewEntrypoint(func() error {
			return errors.New("second entrypoint error")
		}),
	)

	equal(t, err != nil, true)
	equal(t, errors.Is(err, ErrFactoryReturnedError), true)
	equal(t, errors.Is(err, ErrEntrypointReturnedError), true)

	equal(t, normalizeSourceLines(err.Error()), ""+
		"leaf error"+
		"\n"+
		"\nTraceback:"+
		"\n  Factory for *gontainer.testFmtLeaf"+
		"\n  Entrypoint"+
		"\n"+
		"\nsecond entrypoint error"+
		"\n"+
		"\nTraceback:"+
		"\n  Entrypoint")
}

// TestErrorFormatIsMatchesSentinels verifies that errors.Is matches every documented sentinel.
func TestErrorFormatIsMatchesSentinels(t *testing.T) {
	t.Run("FactoryReturnedError", func(t *testing.T) {
		err := Run(
			NewFactory(func() (*testFmtLeaf, error) {
				return nil, errors.New("boom")
			}),
			NewEntrypoint(func(*testFmtLeaf) {}),
		)
		equal(t, errors.Is(err, ErrFactoryReturnedError), true)
	})

	t.Run("EntrypointReturnedError", func(t *testing.T) {
		err := Run(
			NewEntrypoint(func() error {
				return errors.New("boom")
			}),
		)
		equal(t, errors.Is(err, ErrEntrypointReturnedError), true)
	})

	t.Run("DependencyNotResolved", func(t *testing.T) {
		err := Run(
			NewEntrypoint(func(*testFmtLeaf) {}),
		)
		equal(t, errors.Is(err, ErrDependencyNotResolved), true)
	})

	t.Run("CircularDependency", func(t *testing.T) {
		err := Run(
			NewFactory(func(*testFmtMid) *testFmtRootA { return &testFmtRootA{} }),
			NewFactory(func(*testFmtRootA) *testFmtMid { return &testFmtMid{} }),
			NewEntrypoint(func(*testFmtRootA) {}),
		)
		equal(t, errors.Is(err, ErrCircularDependency), true)
	})

	t.Run("FactoryTypeDuplicated", func(t *testing.T) {
		err := Run(
			NewFactory(func() *testFmtLeaf { return &testFmtLeaf{} }),
			NewFactory(func() *testFmtLeaf { return &testFmtLeaf{} }),
			NewEntrypoint(func(*testFmtLeaf) {}),
		)
		equal(t, errors.Is(err, ErrFactoryTypeDuplicated), true)
	})
}

// TestErrorFormatAsUnwrapsToUserError verifies that errors.As reaches a caller-provided error type through the resolve chain.
func TestErrorFormatAsUnwrapsToUserError(t *testing.T) {
	cause := &testFmtTypedErr{msg: "custom user error"}

	err := Run(
		NewFactory(func() (*testFmtLeaf, error) {
			return nil, cause
		}),
		NewFactory(func(*testFmtLeaf) *testFmtMid {
			return &testFmtMid{}
		}),
		NewEntrypoint(func(*testFmtMid) error { return nil }),
	)

	equal(t, err != nil, true)

	var target *testFmtTypedErr
	equal(t, errors.As(err, &target), true)
	equal(t, target.msg, "custom user error")
}

// TestErrorFormatMultipleTagsBothMatch checks that a dynamically resolved factory error keeps both inner and outer sentinels matchable.
func TestErrorFormatMultipleTagsBothMatch(t *testing.T) {
	err := Run(
		NewFactory(func() (*testFmtLeaf, error) {
			return nil, errors.New("leaf factory failed")
		}),
		NewEntrypoint(func(resolver *Resolver) error {
			var leaf *testFmtLeaf
			return resolver.Resolve(&leaf)
		}),
	)

	equal(t, err != nil, true)
	equal(t, errors.Is(err, ErrFactoryReturnedError), true)
	equal(t, errors.Is(err, ErrEntrypointReturnedError), true)
}

// TestErrorFormatPlainMultiLineFallback checks that plain multi-line errors are reproduced verbatim, without bullet prefixes.
func TestErrorFormatPlainMultiLineFallback(t *testing.T) {
	multiLine := errors.New("first line\nsecond line\nthird line")

	err := Run(
		NewFactory(func() (*testFmtLeaf, error) {
			return nil, multiLine
		}),
		NewEntrypoint(func(*testFmtLeaf) {}),
	)

	equal(t, err != nil, true)
	equal(t, normalizeSourceLines(err.Error()), ""+
		"first line"+
		"\nsecond line"+
		"\nthird line"+
		"\n"+
		"\nTraceback:"+
		"\n  Factory for *gontainer.testFmtLeaf"+
		"\n  Entrypoint")
}

// TestErrorFormatSourceLinesAreEmitted verifies every frame carries a distinct "    at <file>:<line>" meta line.
func TestErrorFormatSourceLinesAreEmitted(t *testing.T) {
	err := Run(
		NewFactory(func() (*testFmtLeaf, error) {
			return nil, errors.New("boom")
		}),
		NewFactory(func(*testFmtLeaf) *testFmtMid { return &testFmtMid{} }),
		NewEntrypoint(func(*testFmtMid) {}),
	)

	equal(t, err != nil, true)

	got := err.Error()

	atRe := regexp.MustCompile(`\n {4}at ([^:\n]+):(\d+)`)
	atMatches := atRe.FindAllStringSubmatch(got, -1)
	equal(t, len(atMatches), 3)

	seenLines := map[int]bool{}
	for _, m := range atMatches {
		if m[1] == "" {
			t.Fatalf("expected non-empty file path in 'at' line")
		}
		line, convErr := strconv.Atoi(m[2])
		equal(t, convErr, nil)
		if line <= 0 {
			t.Fatalf("expected positive line number, got %d", line)
		}
		seenLines[line] = true
	}

	if len(seenLines) != 3 {
		t.Fatalf("expected 3 distinct source lines, got %d: %v", len(seenLines), seenLines)
	}
}

// TestErrorFormatCloseSingle verifies that a single close failure renders as "<cause>\n\nSource:\n  Factory for <T>".
func TestErrorFormatCloseSingle(t *testing.T) {
	closeErr := errors.New("sync: file closed")

	err := Run(
		NewFactory(func() (*testFmtLeaf, func() error) {
			return &testFmtLeaf{}, func() error { return closeErr }
		}),
		NewEntrypoint(func(*testFmtLeaf) {}),
	)

	equal(t, err != nil, true)
	equal(t, normalizeSourceLines(err.Error()), ""+
		"sync: file closed"+
		"\n"+
		"\nSource:"+
		"\n  Factory for *gontainer.testFmtLeaf")
	// The user's original error must still be reachable.
	equal(t, errors.Is(err, closeErr), true)
}

// TestErrorFormatCloseMultiple verifies that multiple close failures render as independent blocks in reverse construction order.
func TestErrorFormatCloseMultiple(t *testing.T) {
	errLeaf := errors.New("leaf close failed")
	errMid := errors.New("mid close failed")

	err := Run(
		NewFactory(func() (*testFmtLeaf, func() error) {
			return &testFmtLeaf{}, func() error { return errLeaf }
		}),
		NewFactory(func(*testFmtLeaf) (*testFmtMid, func() error) {
			return &testFmtMid{}, func() error { return errMid }
		}),
		NewEntrypoint(func(*testFmtMid) {}),
	)

	equal(t, err != nil, true)
	// Shutdown order is reverse construction order: the entrypoint's
	// dependency (*testFmtMid) closes first, then *testFmtLeaf.
	equal(t, normalizeSourceLines(err.Error()), ""+
		"mid close failed"+
		"\n"+
		"\nSource:"+
		"\n  Factory for *gontainer.testFmtMid"+
		"\n"+
		"\nleaf close failed"+
		"\n"+
		"\nSource:"+
		"\n  Factory for *gontainer.testFmtLeaf")
	equal(t, errors.Is(err, errLeaf), true)
	equal(t, errors.Is(err, errMid), true)
}

// TestErrorFormatCloseJoinCause verifies that a close error whose cause is an errors.Join renders inner errors as "- " bullets.
func TestErrorFormatCloseJoinCause(t *testing.T) {
	joined := fmt.Errorf("failed to close pool connections: %w", errors.Join(
		errors.New("conn1: write: broken pipe"),
		errors.New("conn2: context canceled"),
	))

	err := Run(
		NewFactory(func() (*testFmtLeaf, func() error) {
			return &testFmtLeaf{}, func() error { return joined }
		}),
		NewEntrypoint(func(*testFmtLeaf) {}),
	)

	equal(t, err != nil, true)
	equal(t, normalizeSourceLines(err.Error()), ""+
		"failed to close pool connections:"+
		"\n- conn1: write: broken pipe"+
		"\n- conn2: context canceled"+
		"\n"+
		"\nSource:"+
		"\n  Factory for *gontainer.testFmtLeaf")
}
