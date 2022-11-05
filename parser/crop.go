package parser

import "fmt"

func Crop(ast *Thrift, reservedNames []string) error {
	names := NewStringSet()
	names.Add(reservedNames...)

	cropper := &cropper{
		ast:      ast,
		reserved: names,
	}

	cropper.crop()

	fmt.Println("Reserved:", cropper.reserved)

	return nil
}

type cropper struct {
	ast      *Thrift
	reserved *StringSet

	moreAdded bool
}

func (c *cropper) crop() error {
	if c.reserved.Empty() {
		return nil
	}

	for _, svc := range c.ast.Services {
		for _, fn := range svc.Functions {
			c.markFunction(fn)
		}
	}

	// collent all reserved names
	c.loopUntilNoMoreAdded()

	// remove all un-reserved names
	c.removeAllUnreserved()

	return nil
}

func (c *cropper) loopUntilNoMoreAdded() {

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
	}
}

func (c *cropper) removeAllUnreserved() {

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

func (c *cropper) markFunction(fn *Function) {
	if !c.reserved.Contains(fn.Name) {
		return
	}

	c.markType(fn.FunctionType)

	for _, field := range fn.Arguments {
		c.markType(field.Type)
	}
}

func (c *cropper) markType(typ *Type) {
	if typ == nil {
		return
	}

	c.addName(typ.Name)
	c.markType(typ.KeyType)
	c.markType(typ.ValueType)
}

func (c *cropper) addName(name string) {
	if c.reserved.Contains(name) {
		return
	}
	// fmt.Printf("adding type %s\n", name)
	c.reserved.Add(name)
	c.moreAdded = true
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
