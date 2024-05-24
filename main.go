package main

import (
	"fmt"
	"go/token"
	"log/slog"
	"os"
	"regexp"
	"sync"

	"github.com/kisielk/errcheck/errcheck"
	"github.com/mgechev/revive/lint"
	"github.com/mgechev/revive/revivelib"
	"github.com/mgechev/revive/rule"
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
		new(rule.ImportsBlocklistRule),
		lint.RuleConfig{
			Arguments: []any{
				"log",
				"google/martian/log",
				"github.com/juju/errors",
				"github.com/pingcap/errors",
				"golang.org/x/exp/slog",
			},
		},
	},
	{
		new(rule.FunctionResultsLimitRule),
		lint.RuleConfig{
			Arguments: []any{
				int64(3),
			},
		},
	},
	{
		new(rule.UnhandledErrorRule),
		lint.RuleConfig{
			Arguments: []any{
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
			Arguments: []any{
				[]any{"call-chain", "method-call", "recover", "return"},
			},
		},
	},
	{
		new(customrules.VarNamingRule),
		lint.RuleConfig{
			Arguments: []any{
				[]any{
					"DANGER_FlushAllKeysFromDatabase",
					"DANGER_FlushAllKeys",
				},
				[]any{
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
			Arguments: []any{
				[]any{
					"DANGER_FlushAllKeysFromDatabase",
					"DANGER_FlushAllKeys",
				},
				[]any{
					"PDF",
					"CSV",
				},
			},
		},
	},
}

func main() {
	if err := run(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
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
	failuresCh, err := revive.Lint(includes...)
	if err != nil {
		return err
	}

	failuresCh2 := make(chan lint.Failure)
	checker := &errcheck.Checker{
		Exclusions: errcheck.Exclusions{
			Packages: []string{
				"fmt",
				"github.com/spf13/cobra",
			},
			SymbolRegexpsByPackage: map[string]*regexp.Regexp{
				"": regexp.MustCompile("Close|MarkFlagRequired|SetLogger"),
			},
			BlankAssignments: true,
			TypeAssertions:   true,
		},
	}
	pkgs, err := checker.LoadPackages(flag.Args()...)
	if err != nil {
		return err
	}
	go func() {
		reported := make(map[token.Position]bool)
		for _, pkg := range pkgs {
			for _, unchecked := range checker.CheckPackage(pkg).UncheckedErrors {
				if reported[unchecked.Pos] {
					continue
				}
				reported[unchecked.Pos] = true

				failuresCh2 <- lint.Failure{
					Confidence: 1,
					RuleName:   "altipla-errcheck",
					Failure:    "unchecked error",
					Position: lint.FailurePosition{
						Start: unchecked.Pos,
					},
				}
			}
		}
		close(failuresCh2)
	}()

	failures, exitCode, err := revive.Format("stylish", multiplex(failuresCh, failuresCh2))
	if err != nil {
		return err
	}
	if failures != "" {
		fmt.Println(failures)
	}
	os.Exit(exitCode)

	return nil
}

func multiplex(channels ...<-chan lint.Failure) <-chan lint.Failure {
	var wg sync.WaitGroup
	out := make(chan lint.Failure)

	output := func(c <-chan lint.Failure) {
		for failure := range c {
			out <- failure
		}
		wg.Done()
	}

	wg.Add(len(channels))
	for _, c := range channels {
		go output(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}
