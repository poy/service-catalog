/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package class

import (
	"github.com/poy/service-catalog/cmd/svcat/command"
	"github.com/poy/service-catalog/cmd/svcat/output"
	"github.com/poy/service-catalog/pkg/svcat/service-catalog"
	"github.com/spf13/cobra"
)

type getCmd struct {
	*command.Namespaced
	*command.Scoped
	*command.Formatted
	lookupByKubeName bool
	kubeName         string
	name             string
}

// NewGetCmd builds a "svcat get classes" command
func NewGetCmd(cxt *command.Context) *cobra.Command {
	getCmd := &getCmd{
		Namespaced: command.NewNamespaced(cxt),
		Scoped:     command.NewScoped(),
		Formatted:  command.NewFormatted(),
	}
	cmd := &cobra.Command{
		Use:     "classes [NAME]",
		Aliases: []string{"class", "cl"},
		Short:   "List classes, optionally filtered by name, scope or namespace",
		Example: command.NormalizeExamples(`
  svcat get classes
  svcat get classes --scope cluster
  svcat get classes --scope namespace --namespace dev
  svcat get class mysqldb
  svcat get class --kube-name 997b8372-8dac-40ac-ae65-758b4a5075a5
`),
		PreRunE: command.PreRunE(getCmd),
		RunE:    command.RunE(getCmd),
	}
	cmd.Flags().BoolVarP(
		&getCmd.lookupByKubeName,
		"kube-name",
		"k",
		false,
		"Whether or not to get the class by its Kubernetes name (the default is by external name)",
	)
	getCmd.AddOutputFlags(cmd.Flags())
	getCmd.AddNamespaceFlags(cmd.Flags(), true)
	getCmd.AddScopedFlags(cmd.Flags(), true)
	return cmd
}

func (c *getCmd) Validate(args []string) error {
	if len(args) > 0 {
		if c.lookupByKubeName {
			c.kubeName = args[0]
		} else {
			c.name = args[0]
		}
	}

	return nil
}

func (c *getCmd) Run() error {
	if c.kubeName == "" && c.name == "" {
		return c.getAll()
	}

	return c.get()
}

func (c *getCmd) getAll() error {
	opts := servicecatalog.ScopeOptions{
		Namespace: c.Namespace,
		Scope:     c.Scope,
	}
	classes, err := c.App.RetrieveClasses(opts)
	if err != nil {
		return err
	}

	output.WriteClassList(c.Output, c.OutputFormat, classes...)
	return nil
}

func (c *getCmd) get() error {
	var class servicecatalog.Class
	var err error

	if c.lookupByKubeName {
		class, err = c.App.RetrieveClassByID(c.kubeName)
	} else if c.name != "" {
		class, err = c.App.RetrieveClassByName(c.name, servicecatalog.ScopeOptions{Scope: c.Scope, Namespace: c.Namespace})
	}
	if err != nil {
		return err
	}

	output.WriteClass(c.Output, c.OutputFormat, class)
	return nil
}
