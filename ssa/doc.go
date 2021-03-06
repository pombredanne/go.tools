// Package ssa defines a representation of the elements of Go programs
// (packages, types, functions, variables and constants) using a
// static single-assignment (SSA) form intermediate representation
// (IR) for the bodies of functions.
//
// THIS INTERFACE IS EXPERIMENTAL AND IS LIKELY TO CHANGE.
//
// For an introduction to SSA form, see
// http://en.wikipedia.org/wiki/Static_single_assignment_form.
// This page provides a broader reading list:
// http://www.dcs.gla.ac.uk/~jsinger/ssa.html.
//
// The level of abstraction of the SSA form is intentionally close to
// the source language to facilitate construction of source analysis
// tools.  It is not primarily intended for machine code generation.
//
// All looping, branching and switching constructs are replaced with
// unstructured control flow.  We may add higher-level control flow
// primitives in the future to facilitate constant-time dispatch of
// switch statements, for example.
//
// Builder encapsulates the tasks of type-checking (using go/types)
// abstract syntax trees (as defined by go/ast) for the source files
// comprising a Go program, and the conversion of each function from
// Go ASTs to the SSA representation.
//
// By supplying an instance of the SourceLocator function prototype,
// clients may control how the builder locates, loads and parses Go
// sources files for imported packages.  This package provides
// MakeGoBuildLoader, which creates a loader that uses go/build to
// locate packages in the Go source distribution, and go/parser to
// parse them.
//
// The builder initially builds a naive SSA form in which all local
// variables are addresses of stack locations with explicit loads and
// stores.  Registerisation of eligible locals and φ-node insertion
// using dominance and dataflow are then performed as a second pass
// called "lifting" to improve the accuracy and performance of
// subsequent analyses; this pass can be skipped by setting the
// NaiveForm builder flag.
//
// The primary interfaces of this package are:
//
//    - Member: a named member of a Go package.
//    - Value: an expression that yields a value.
//    - Instruction: a statement that consumes values and performs computation.
//
// A computation that yields a result implements both the Value and
// Instruction interfaces.  The following table shows for each
// concrete type which of these interfaces it implements.
//
//                      Value?          Instruction?    Member?
//   *Alloc             ✔               ✔
//   *BinOp             ✔               ✔
//   *Builtin           ✔               ✔
//   *Call              ✔               ✔
//   *Capture           ✔
//   *ChangeInterface   ✔               ✔
//   *ChangeType        ✔               ✔
//   *Constant                                          ✔ (const)
//   *Convert           ✔               ✔
//   *Defer                             ✔
//   *Extract           ✔               ✔
//   *Field             ✔               ✔
//   *FieldAddr         ✔               ✔
//   *Function          ✔                               ✔ (func)
//   *Global            ✔                               ✔ (var)
//   *Go                                ✔
//   *If                                ✔
//   *Index             ✔               ✔
//   *IndexAddr         ✔               ✔
//   *Jump                              ✔
//   *Literal           ✔
//   *Lookup            ✔               ✔
//   *MakeChan          ✔               ✔
//   *MakeClosure       ✔               ✔
//   *MakeInterface     ✔               ✔
//   *MakeMap           ✔               ✔
//   *MakeSlice         ✔               ✔
//   *MapUpdate                         ✔
//   *Next              ✔               ✔
//   *Panic                             ✔
//   *Parameter         ✔
//   *Phi               ✔               ✔
//   *Range             ✔               ✔
//   *Ret                               ✔
//   *RunDefers                         ✔
//   *Select            ✔               ✔
//   *Slice             ✔               ✔
//   *Type                                              ✔ (type)
//   *TypeAssert        ✔               ✔
//   *UnOp              ✔               ✔
//
// Other key types in this package include: Program, Package, Function
// and BasicBlock.
//
// The program representation constructed by this package is fully
// resolved internally, i.e. it does not rely on the names of Values,
// Packages, Functions, Types or BasicBlocks for the correct
// interpretation of the program.  Only the identities of objects and
// the topology of the SSA and type graphs are semantically
// significant.  (There is one exception: Ids, used to identify field
// and method names, contain strings.)  Avoidance of name-based
// operations simplifies the implementation of subsequent passes and
// can make them very efficient.  Many objects are nonetheless named
// to aid in debugging, but it is not essential that the names be
// either accurate or unambiguous.  The public API exposes a number of
// name-based maps for client convenience.
//
// TODO(adonovan): Consider the exceptional control-flow implications
// of defer and recover().
//
// TODO(adonovan): Consider how token.Pos source location information
// should be made available generally.  Currently it is only present
// in package Members and selected Instructions for which there is a
// direct source correspondence.  We'll need to work harder to tie all
// defs/uses of named variables together, esp. because SSA splits them
// into separate webs.
//
// TODO(adonovan): it is practically impossible for clients to
// construct well-formed SSA functions/packages/programs directly; we
// assume this is the job of the ssa.Builder alone.
// Nonetheless it may be wise to give clients a little more
// flexibility.  For example, analysis tools may wish to construct a
// fake ssa.Function for the root of the callgraph, a fake "reflect"
// package, etc.
//
package ssa
