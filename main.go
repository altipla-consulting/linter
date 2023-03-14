package main

import (
	"fmt"
	"os"

	"github.com/mgechev/revive/lint"
	"github.com/mgechev/revive/revivelib"
	"github.com/mgechev/revive/rule"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"

	"github.com/altipla-consulting/linter/customrules"
)

type ruleConfig struct {
	Rule   lint.Rule
	Config lint.RuleConfig
}

var standardRules = []ruleConfig{
	{new(rule.ArgumentsLimitRule), lint.RuleConfig{Disabled: true}},
	{new(rule.CyclomaticRule), lint.RuleConfig{Disabled: true}},
	{new(rule.FileHeaderRule), lint.RuleConfig{Disabled: true}},
	{new(rule.DeepExitRule), lint.RuleConfig{Disabled: true}},
	{new(rule.UnusedParamRule), lint.RuleConfig{Disabled: true}},
	{new(rule.AddConstantRule), lint.RuleConfig{Disabled: true}},
	{new(rule.FlagParamRule), lint.RuleConfig{Disabled: true}},
	{new(rule.MaxPublicStructsRule), lint.RuleConfig{Disabled: true}},
	{new(rule.LineLengthLimitRule), lint.RuleConfig{Disabled: true}},
	{new(rule.UnusedReceiverRule), lint.RuleConfig{Disabled: true}},
	{new(rule.CognitiveComplexityRule), lint.RuleConfig{Disabled: true}},
	{new(rule.FunctionLength), lint.RuleConfig{Disabled: true}},
	{new(rule.ExportedRule), lint.RuleConfig{Disabled: true}},
	{new(rule.VarNamingRule), lint.RuleConfig{Disabled: true}},
	{
		new(rule.ImportsBlacklistRule),
		lint.RuleConfig{
			Arguments: []interface{}{
				"log",
				"google/martian/log",
				"github.com/juju/errors",
			},
		},
	},
	{
		new(rule.FunctionResultsLimitRule),
		lint.RuleConfig{
			Arguments: []interface{}{
				int64(3),
			},
		},
	},
	{
		new(rule.UnhandledErrorRule),
		lint.RuleConfig{
			Arguments: []interface{}{
				"fmt.Fprint",
				"fmt.Fprintf",
				"fmt.Fprintln",
				"fmt.Println",
				"fmt.Print",
				"fmt.Printf",
				"rand.Read",
			},
		},
	},
	{
		new(rule.DeferRule),
		lint.RuleConfig{
			Arguments: []interface{}{
				[]interface{}{"call-chain", "method-call", "recover", "return"},
			},
		},
	},
	{
		new(customrules.VarNamingRule),
		lint.RuleConfig{
			Arguments: []interface{}{
				[]interface{}{
					"DANGER_FlushAllKeysFromDatabase",
					"DANGER_FlushAllKeys",
				},
				[]interface{}{
					"PDF",
					"CSV",
				},
			},
		},
	},
}

var customRules = []ruleConfig{
	{
		new(customrules.MultilineIfRule),
		lint.RuleConfig{},
	},
	{
		new(customrules.VarNamingRule),
		lint.RuleConfig{
			Arguments: []interface{}{
				[]interface{}{
					"DANGER_FlushAllKeysFromDatabase",
					"DANGER_FlushAllKeys",
				},
				[]interface{}{
					"PDF",
					"CSV",
				},
			},
		},
	},
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	flag.Parse()

	cnf := &lint.Config{
		EnableAllRules: true,
		Severity:       lint.SeverityError,
		Confidence:     0.8,
		Rules:          make(lint.RulesConfig),
	}
	for _, r := range standardRules {
		cnf.Rules[r.Rule.Name()] = r.Config
	}
	extra := make([]revivelib.ExtraRule, len(customRules))
	for i, r := range customRules {
		extra[i] = revivelib.NewExtraRule(r.Rule, r.Config)
	}

	revive, err := revivelib.New(cnf, true, 0, extra...)
	if err != nil {
		return err
	}

	var includes []*revivelib.LintPattern
	for _, arg := range flag.Args() {
		includes = append(includes, revivelib.Include(arg))
	}
	failuresChan, err := revive.Lint(includes...)
	if err != nil {
		return err
	}

	failures, exitCode, err := revive.Format("stylish", failuresChan)
	if err != nil {
		return err
	}
	fmt.Println(failures)
	os.Exit(exitCode)

	return nil
}
