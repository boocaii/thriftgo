package parser

// RName: Reserved Name

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

var (
	defaultConfigFileName = "pruner.yml"
	config                *PruneConfig

	ast2FilePruner = make(map[*Thrift]*filePruner)
)

// Prune removes all symbols, except specified in argument ro/reserve_only or pruner.yml
func Prune(ast *Thrift, rNames []string) {
	//if len(rNames) == 0 {
	//	return
	//}

	config = loadCropperConfigFile(defaultConfigFileName)

	ast2FilePruner[ast] = &filePruner{ast: ast, reserved: NewStringSet(rNames...)}
	collectRecursively(ast)
	pruneRecursively(ast)

	saveCropperConfigFile(config, defaultConfigFileName)
}

func collectRecursively(ast *Thrift) {
	if ast == nil {
		return
	}
	fmt.Printf("ast: %s\n", ast.Filename)
	c := ast2FilePruner[ast]
	if c == nil {
		c = &filePruner{
			ast:      ast,
			reserved: NewStringSet(),
		}
		ast2FilePruner[ast] = c
	}

	c.collectRNames()
	for _, include := range ast.Includes {
		includePrefix := fileNameWithoutExt(include.Path)
		fmt.Printf("path: %s, includePrefix: %s\n", include.Path, includePrefix)
		collectRecursively(include.Reference)
	}
}

func pruneRecursively(ast *Thrift) {
	ast.Name2Category = nil

	c := ast2FilePruner[ast]
	fmt.Printf("File: %s, Reserved: %+v\n", ast.Filename, c.reserved)

	// prune it
	c.removeAllUnreserved()

	reservedIncludes := []*Include{}
	for _, include := range ast.Includes {
		if p := ast2FilePruner[include.Reference]; p != nil && p.reserved.Empty() {
			continue
		}
		pruneRecursively(include.Reference)
		reservedIncludes = append(reservedIncludes, include)
	}
	ast.Includes = reservedIncludes
}

type filePruner struct {
	ast       *Thrift
	reserved  *StringSet
	gitURL    string
	moreAdded bool
}

// collect reserved names
func (c *filePruner) collectRNames() {
	c.findGit()

	// read names from config
	if file := config.findFile(c.gitURL, c.ast.Filename); file != nil {
		c.reserved.Add(file.Names...)
	}

	for _, svc := range c.ast.Services {
		for _, fn := range svc.Functions {
			c.markFunction(fn)
		}
	}

	c.loopUntilNoMoreAdded()

	c.toConfig()
}

func (c *filePruner) toConfig() {
	if c.reserved.Empty() {
		return
	}
	file := config.findFile(c.gitURL, c.ast.Filename)
	if file == nil {
		file = config.initFile(c.gitURL, c.ast.Filename)
	}

	c.reserved.Add(file.Names...)
	file.Names = c.reserved.ToSlice()
}

func (c *filePruner) loopUntilNoMoreAdded() {

	c.moreAdded = true
	for c.moreAdded {
		c.moreAdded = false

		for _, v := range c.ast.GetStructLikes() {
			if c.reserved.Contains(v.Name) {
				for _, field := range v.Fields {
					c.markType(field.Type)
				}
			}
		}

		for _, v := range c.ast.GetTypedefs() {
			if c.reserved.Contains(v.Alias) {
				c.markType(v.Type)
			}
		}

		for _, v := range c.ast.GetConstants() {
			if c.reserved.Contains(v.Name) {
				c.markType(v.Type)
			}
		}
	}
}

func (c *filePruner) removeAllUnreserved() {

	// remove functions
	for _, svc := range c.ast.Services {
		ss := []*Function{}
		for _, fn := range svc.Functions {
			if c.reserved.Contains(fn.Name) {
				ss = append(ss, fn)
			}
		}
		svc.Functions = ss
	}

	// remove structs
	c.ast.Structs = filterStructLike(c.ast.Structs, c.reserved)
	c.ast.Unions = filterStructLike(c.ast.Unions, c.reserved)
	c.ast.Exceptions = filterStructLike(c.ast.Exceptions, c.reserved)

	// remove typedefs
	{
		vs := []*Typedef{}
		for _, v := range c.ast.Typedefs {
			if c.reserved.Contains(v.Alias) {
				vs = append(vs, v)
			}
		}
		c.ast.Typedefs = vs
	}

	// remove enums
	{
		vs := []*Enum{}
		for _, v := range c.ast.Enums {
			if c.reserved.Contains(v.Name) {
				vs = append(vs, v)
			}
		}
		c.ast.Enums = vs
	}

}

func (c *filePruner) markFunction(fn *Function) {
	if !c.reserved.Contains(fn.Name) {
		return
	}

	c.markType(fn.FunctionType)

	for _, field := range fn.Arguments {
		c.markType(field.Type)
	}
}

func (c *filePruner) markType(typ *Type) {
	if typ == nil {
		return
	}

	fmt.Printf("type.Name: %s\n", typ.Name)
	c.addName(typ)
	c.markType(typ.KeyType)
	c.markType(typ.ValueType)
}

func (c *filePruner) addName(typ *Type) {
	if typ == nil {
		return
	}
	if c.reserved.Contains(typ.Name) {
		return
	}
	// fmt.Printf("adding type %s\n", name)
	c.reserved.Add(typ.Name)
	c.checkAndAddIncludeName(typ)
	c.moreAdded = true
}

// check symbols from include
func (c *filePruner) checkAndAddIncludeName(typ *Type) {
	if ref := typ.GetReference(); ref != nil {
		inc := c.ast.Includes[ref.Index]
		if p := ast2FilePruner[inc.Reference]; p == nil {
			ast2FilePruner[inc.Reference] = &filePruner{ast: inc.Reference, reserved: NewStringSet()}
		}
		if splits := strings.Split(typ.Name, "."); len(splits) == 2 {
			ast2FilePruner[inc.Reference].reserved.Add(splits[1])
		}
	}
}

func (c *filePruner) findGit() {
	gitURL, err := os.Getwd()
	defer func() {
		c.gitURL = gitURL
	}()

	r, err := git.PlainOpenWithOptions(c.ast.Filename, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil && !errors.Is(err, git.ErrRepositoryNotExists) {
		panic(err)
	}
	if r == nil {
		return
	}
	remote, err := r.Remote("origin")
	if err != nil {
		if errors.Is(err, git.ErrRemoteNotFound) {
			fmt.Println("Please set git remote of", c.ast.Filename)
			os.Exit(1)
		}
	}
	gitURL = remote.Config().URLs[0]
	//fmt.Printf("%s, git root: %s\n", c.ast.Filename, c.gitURL)
}

func filterStructLike(inputs []*StructLike, set *StringSet) []*StructLike {
	output := []*StructLike{}
	for _, input := range inputs {
		if set.Contains(input.Name) {
			output = append(output, input)
		}
	}
	return output
}

// PruneConfig - config file
type PruneConfig struct {
	ReservedGits []*ReservedGit `yaml:"reserved_gits"`
}

type ReservedGit struct {
	Git   string
	Files []*ReservedFile
}

type ReservedFile struct {
	File  string
	Names []string
}

func (c *PruneConfig) initFile(git, file string) *ReservedFile {
	for _, git_ := range c.ReservedGits {
		if git_.Git == git {
			for _, file_ := range git_.Files {
				if file_.File == file {
					return file_
				}
			}

			// not found
			file_ := &ReservedFile{File: file}
			git_.Files = append(git_.Files, file_)
			return file_
		}
	}

	git_ := &ReservedGit{Git: git}
	file_ := &ReservedFile{File: file}
	git_.Files = append(git_.Files, file_)
	c.ReservedGits = append(c.ReservedGits, git_)
	return file_
}

func (c *PruneConfig) findFile(git, file string) *ReservedFile {
	for _, git_ := range c.ReservedGits {
		if git_.Git == git {
			for _, file_ := range git_.Files {
				if file_.File == file {
					return file_
				}
			}
		}
	}
	return nil
}

func loadCropperConfigFile(path string) *PruneConfig {
	if path == "" {
		path = defaultConfigFileName
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &PruneConfig{}
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	config := PruneConfig{}
	err = yaml.Unmarshal(bytes, &config)
	if err != nil {
		panic(err)
	}
	return &config
}

func saveCropperConfigFile(config *PruneConfig, path string) {
	if config == nil {
		return
	}
	if path == "" {
		path = defaultConfigFileName
	}

	bytes, err := yaml.Marshal(config)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(path, bytes, 0664)
	if err != nil {
		panic(err)
	}
}
