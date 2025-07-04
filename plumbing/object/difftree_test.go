package object

import (
	"fmt"
	"sort"
	"testing"

	fixtures "github.com/go-git/go-git-fixtures/v5"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/cache"
	"github.com/go-git/go-git/v6/plumbing/filemode"
	"github.com/go-git/go-git/v6/plumbing/format/packfile"
	"github.com/go-git/go-git/v6/plumbing/storer"
	"github.com/go-git/go-git/v6/storage/filesystem"
	"github.com/go-git/go-git/v6/storage/memory"
	"github.com/go-git/go-git/v6/utils/merkletrie"
	"github.com/stretchr/testify/suite"
)

type DiffTreeSuite struct {
	suite.Suite
	Storer  storer.EncodedObjectStorer
	Fixture *fixtures.Fixture
	cache   map[string]storer.EncodedObjectStorer
}

func (s *DiffTreeSuite) SetupSuite() {
	s.Fixture = fixtures.Basic().One()
	sto := filesystem.NewStorage(s.Fixture.DotGit(), cache.NewObjectLRUDefault())
	s.Storer = sto
	s.cache = make(map[string]storer.EncodedObjectStorer)
}

func (s *DiffTreeSuite) commitFromStorer(sto storer.EncodedObjectStorer,
	h plumbing.Hash) *Commit {

	commit, err := GetCommit(sto, h)
	s.NoError(err)
	return commit
}

func (s *DiffTreeSuite) storageFromPackfile(f *fixtures.Fixture) storer.EncodedObjectStorer {
	sto, ok := s.cache[f.URL]
	if ok {
		return sto
	}

	storer := memory.NewStorage()

	pf := f.Packfile()
	defer pf.Close()

	if err := packfile.UpdateObjectStorage(storer, pf); err != nil {
		panic(err)
	}

	s.cache[f.URL] = storer
	return storer
}

func TestDiffTreeSuite(t *testing.T) {
	suite.Run(t, new(DiffTreeSuite))
}

type expectChange struct {
	Action merkletrie.Action
	Name   string
}

func assertChanges(a Changes, s *DiffTreeSuite) {
	for _, changes := range a {
		action, err := changes.Action()
		s.NoError(err)
		switch action {
		case merkletrie.Insert:
			s.Nil(changes.From.Tree)
			s.NotNil(changes.To.Tree)
		case merkletrie.Delete:
			s.NotNil(changes.From.Tree)
			s.Nil(changes.To.Tree)
		case merkletrie.Modify:
			s.NotNil(changes.From.Tree)
			s.NotNil(changes.To.Tree)
		default:
			s.Fail("unknown action:", action)
		}
	}
}

func equalChanges(a Changes, b []expectChange, s *DiffTreeSuite) bool {
	if len(a) != len(b) {
		return false
	}

	sort.Sort(a)

	for i, va := range a {
		vb := b[i]
		action, err := va.Action()
		s.NoError(err)
		if action != vb.Action || va.name() != vb.Name {
			return false
		}
	}

	return true
}

func (s *DiffTreeSuite) TestDiffTree() {
	for i, t := range []struct {
		repository string         // the repo name as in localRepos
		commit1    string         // the commit of the first tree
		commit2    string         // the commit of the second tree
		expected   []expectChange // the expected list of []changeExpect
	}{
		{
			"https://github.com/dezfowler/LiteMock.git",
			"",
			"",
			[]expectChange{},
		},
		{
			"https://github.com/dezfowler/LiteMock.git",
			"b7965eaa2c4f245d07191fe0bcfe86da032d672a",
			"b7965eaa2c4f245d07191fe0bcfe86da032d672a",
			[]expectChange{},
		},
		{
			"https://github.com/dezfowler/LiteMock.git",
			"",
			"b7965eaa2c4f245d07191fe0bcfe86da032d672a",
			[]expectChange{
				{Action: merkletrie.Insert, Name: "README"},
			},
		},
		{
			"https://github.com/dezfowler/LiteMock.git",
			"b7965eaa2c4f245d07191fe0bcfe86da032d672a",
			"",
			[]expectChange{
				{Action: merkletrie.Delete, Name: "README"},
			},
		},
		{
			"https://github.com/githubtraining/example-branches.git",
			"",
			"f0eb272cc8f77803478c6748103a1450aa1abd37",
			[]expectChange{
				{Action: merkletrie.Insert, Name: "README.md"},
			},
		},
		{
			"https://github.com/githubtraining/example-branches.git",
			"f0eb272cc8f77803478c6748103a1450aa1abd37",
			"",
			[]expectChange{
				{Action: merkletrie.Delete, Name: "README.md"},
			},
		},
		{
			"https://github.com/githubtraining/example-branches.git",
			"f0eb272cc8f77803478c6748103a1450aa1abd37",
			"f0eb272cc8f77803478c6748103a1450aa1abd37",
			[]expectChange{},
		},
		{
			"https://github.com/github/gem-builder.git",
			"",
			"9608eed92b3839b06ebf72d5043da547de10ce85",
			[]expectChange{
				{Action: merkletrie.Insert, Name: "README"},
				{Action: merkletrie.Insert, Name: "gem_builder.rb"},
				{Action: merkletrie.Insert, Name: "gem_eval.rb"},
			},
		},
		{
			"https://github.com/github/gem-builder.git",
			"9608eed92b3839b06ebf72d5043da547de10ce85",
			"",
			[]expectChange{
				{Action: merkletrie.Delete, Name: "README"},
				{Action: merkletrie.Delete, Name: "gem_builder.rb"},
				{Action: merkletrie.Delete, Name: "gem_eval.rb"},
			},
		},
		{
			"https://github.com/github/gem-builder.git",
			"9608eed92b3839b06ebf72d5043da547de10ce85",
			"9608eed92b3839b06ebf72d5043da547de10ce85",
			[]expectChange{},
		},
		{
			"https://github.com/toqueteos/ts3.git",
			"",
			"764e914b75d6d6df1fc5d832aa9840f590abf1bb",
			[]expectChange{
				{Action: merkletrie.Insert, Name: "README.markdown"},
				{Action: merkletrie.Insert, Name: "examples/bot.go"},
				{Action: merkletrie.Insert, Name: "examples/raw_shell.go"},
				{Action: merkletrie.Insert, Name: "helpers.go"},
				{Action: merkletrie.Insert, Name: "ts3.go"},
			},
		},
		{
			"https://github.com/toqueteos/ts3.git",
			"764e914b75d6d6df1fc5d832aa9840f590abf1bb",
			"",
			[]expectChange{
				{Action: merkletrie.Delete, Name: "README.markdown"},
				{Action: merkletrie.Delete, Name: "examples/bot.go"},
				{Action: merkletrie.Delete, Name: "examples/raw_shell.go"},
				{Action: merkletrie.Delete, Name: "helpers.go"},
				{Action: merkletrie.Delete, Name: "ts3.go"},
			},
		},
		{
			"https://github.com/toqueteos/ts3.git",
			"764e914b75d6d6df1fc5d832aa9840f590abf1bb",
			"764e914b75d6d6df1fc5d832aa9840f590abf1bb",
			[]expectChange{},
		},
		{
			"https://github.com/github/gem-builder.git",
			"9608eed92b3839b06ebf72d5043da547de10ce85",
			"6c41e05a17e19805879689414026eb4e279f7de0",
			[]expectChange{
				{Action: merkletrie.Modify, Name: "gem_eval.rb"},
			},
		},
		{
			"https://github.com/github/gem-builder.git",
			"6c41e05a17e19805879689414026eb4e279f7de0",
			"89be3aac2f178719c12953cc9eaa23441f8d9371",
			[]expectChange{
				{Action: merkletrie.Modify, Name: "gem_eval.rb"},
				{Action: merkletrie.Insert, Name: "gem_eval_test.rb"},
				{Action: merkletrie.Insert, Name: "security.rb"},
				{Action: merkletrie.Insert, Name: "security_test.rb"},
			},
		},
		{
			"https://github.com/github/gem-builder.git",
			"89be3aac2f178719c12953cc9eaa23441f8d9371",
			"597240b7da22d03ad555328f15abc480b820acc0",
			[]expectChange{
				{Action: merkletrie.Modify, Name: "gem_eval.rb"},
			},
		},
		{
			"https://github.com/github/gem-builder.git",
			"597240b7da22d03ad555328f15abc480b820acc0",
			"0260380e375d2dd0e1a8fcab15f91ce56dbe778e",
			[]expectChange{
				{Action: merkletrie.Modify, Name: "gem_eval.rb"},
				{Action: merkletrie.Modify, Name: "gem_eval_test.rb"},
				{Action: merkletrie.Insert, Name: "lazy_dir.rb"},
				{Action: merkletrie.Insert, Name: "lazy_dir_test.rb"},
				{Action: merkletrie.Modify, Name: "security.rb"},
				{Action: merkletrie.Modify, Name: "security_test.rb"},
			},
		},
		{
			"https://github.com/github/gem-builder.git",
			"0260380e375d2dd0e1a8fcab15f91ce56dbe778e",
			"597240b7da22d03ad555328f15abc480b820acc0",
			[]expectChange{
				{Action: merkletrie.Modify, Name: "gem_eval.rb"},
				{Action: merkletrie.Modify, Name: "gem_eval_test.rb"},
				{Action: merkletrie.Delete, Name: "lazy_dir.rb"},
				{Action: merkletrie.Delete, Name: "lazy_dir_test.rb"},
				{Action: merkletrie.Modify, Name: "security.rb"},
				{Action: merkletrie.Modify, Name: "security_test.rb"},
			},
		},
		{
			"https://github.com/github/gem-builder.git",
			"0260380e375d2dd0e1a8fcab15f91ce56dbe778e",
			"ca9fd470bacb6262eb4ca23ee48bb2f43711c1ff",
			[]expectChange{
				{Action: merkletrie.Modify, Name: "gem_eval.rb"},
				{Action: merkletrie.Modify, Name: "security.rb"},
				{Action: merkletrie.Modify, Name: "security_test.rb"},
			},
		},
		{
			"https://github.com/github/gem-builder.git",
			"fe3c86745f887c23a0d38c85cfd87ca957312f86",
			"b7e3f636febf7a0cd3ab473b6d30081786d2c5b6",
			[]expectChange{
				{Action: merkletrie.Modify, Name: "gem_eval.rb"},
				{Action: merkletrie.Modify, Name: "gem_eval_test.rb"},
				{Action: merkletrie.Insert, Name: "git_mock"},
				{Action: merkletrie.Modify, Name: "lazy_dir.rb"},
				{Action: merkletrie.Modify, Name: "lazy_dir_test.rb"},
				{Action: merkletrie.Modify, Name: "security.rb"},
			},
		},
		{
			"https://github.com/rumpkernel/rumprun-xen.git",
			"1831e47b0c6db750714cd0e4be97b5af17fb1eb0",
			"51d8515578ea0c88cc8fc1a057903675cf1fc16c",
			[]expectChange{
				{Action: merkletrie.Modify, Name: "Makefile"},
				{Action: merkletrie.Modify, Name: "netbsd_init.c"},
				{Action: merkletrie.Modify, Name: "rumphyper_stubs.c"},
				{Action: merkletrie.Delete, Name: "sysproxy.c"},
			},
		},
		{
			"https://github.com/rumpkernel/rumprun-xen.git",
			"1831e47b0c6db750714cd0e4be97b5af17fb1eb0",
			"e13e678f7ee9badd01b120889e0ec5fdc8ae3802",
			[]expectChange{
				{Action: merkletrie.Modify, Name: "app-tools/rumprun"},
			},
		},
	} {
		f := fixtures.ByURL(t.repository).One()
		sto := s.storageFromPackfile(f)

		var tree1, tree2 *Tree
		var err error
		if t.commit1 != "" {
			tree1, err = s.commitFromStorer(sto,
				plumbing.NewHash(t.commit1)).Tree()
			s.NoError(err,
				fmt.Sprintf("subtest %d: unable to retrieve tree from commit %s and repo %s: %s", i, t.commit1, t.repository, err))
		}

		if t.commit2 != "" {
			tree2, err = s.commitFromStorer(sto,
				plumbing.NewHash(t.commit2)).Tree()
			s.NoError(err,
				fmt.Sprintf("subtest %d: unable to retrieve tree from commit %s and repo %s", i, t.commit2, t.repository))
		}

		obtained, err := DiffTree(tree1, tree2)
		s.NoError(err,
			fmt.Sprintf("subtest %d: unable to calculate difftree: %s", i, err))
		obtainedFromMethod, err := tree1.Diff(tree2)
		s.NoError(err,
			fmt.Sprintf("subtest %d: unable to calculate difftree: %s. Result calling Diff method from Tree object returns an error", i, err))

		s.Equal(obtainedFromMethod, obtained)

		s.True(equalChanges(obtained, t.expected, s),
			fmt.Sprintf("subtest:%d\nrepo=%s\ncommit1=%s\ncommit2=%s\nexpected=%s\nobtained=%s",
				i, t.repository, t.commit1, t.commit2, t.expected, obtained))

		assertChanges(obtained, s)
	}
}

func (s *DiffTreeSuite) TestIssue279() {
	// treeNoders should have the same hash when their mode is
	// filemode.Deprecated and filemode.Regular.
	a := &treeNoder{
		hash: plumbing.NewHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		mode: filemode.Regular,
	}
	b := &treeNoder{
		hash: plumbing.NewHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		mode: filemode.Deprecated,
	}
	s.Equal(b.Hash(), a.Hash())

	// yet, they should have different hashes if their contents change.
	aa := &treeNoder{
		hash: plumbing.NewHash("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
		mode: filemode.Regular,
	}
	s.NotEqual(aa.Hash(), a.Hash())
	bb := &treeNoder{
		hash: plumbing.NewHash("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
		mode: filemode.Deprecated,
	}
	s.NotEqual(bb.Hash(), b.Hash())
}
