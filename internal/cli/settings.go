package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
)

func (a *App) runSettings(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "szr: settings does not accept arguments")
		return 2
	}
	return a.runSettingsInteractive(os.Stdin, os.Stdout, os.Stderr)
}

func (a *App) runSettingsInteractive(stdin io.Reader, stdout, stderr io.Writer) int {
	reader := bufio.NewReader(stdin)
	for {
		a.printSettingsMenu(stdout, a.config, a.paths.ConfigFile)

		choice, ok, err := readSettingsLine(reader)
		if err != nil {
			fmt.Fprintf(stderr, "szr: failed to read settings input: %v\n", err)
			return 1
		}
		if !ok {
			fmt.Fprintln(stdout, "settings: exiting")
			return 0
		}
		exitCode, done := a.handleSettingsChoice(choice, reader, stdout, stderr)
		if exitCode != 0 || done {
			return exitCode
		}
	}
}

func (a *App) handleSettingsChoice(choice string, reader *bufio.Reader, stdout, stderr io.Writer) (int, bool) {
	switch choice {
	case "1":
		return a.updateBooleanSetting(reader, stdout, stderr, "update checks", a.config.UpdateCheck.Enabled, "settings: update checks unchanged", func(cfg *config.Config, value bool) string {
			cfg.UpdateCheck.Enabled = value
			return enabledLabel(cfg.UpdateCheck.Enabled)
		})
	case "2":
		return a.updateBooleanSetting(reader, stdout, stderr, "auto update", a.config.UpdateCheck.AutoUpdate, "settings: auto update unchanged", func(cfg *config.Config, value bool) string {
			cfg.UpdateCheck.AutoUpdate = value
			if cfg.UpdateCheck.AutoUpdate {
				cfg.UpdateCheck.Enabled = true
			}
			return enabledLabel(cfg.UpdateCheck.AutoUpdate)
		})
	case "3":
		return a.updatePositiveIntSetting(reader, stdout, stderr, "update interval hours", "update interval", "settings: interval unchanged", func(cfg *config.Config, value int) string {
			cfg.UpdateCheck.IntervalHours = value
			return fmt.Sprintf("%dh", cfg.UpdateCheck.IntervalHours)
		})
	case "4":
		return a.updateBooleanSetting(reader, stdout, stderr, "tee on failure", a.config.TeeOnFailure, "settings: tee on failure unchanged", func(cfg *config.Config, value bool) string {
			cfg.TeeOnFailure = value
			return enabledLabel(cfg.TeeOnFailure)
		})
	case "5":
		return a.updatePositiveIntSetting(reader, stdout, stderr, "max preview lines", "max preview lines", "settings: max preview lines unchanged", func(cfg *config.Config, value int) string {
			cfg.MaxPreviewLines = value
			return fmt.Sprintf("%d", cfg.MaxPreviewLines)
		})
	case "6":
		return a.updatePositiveIntSetting(reader, stdout, stderr, "max match groups", "max match groups", "settings: max match groups unchanged", func(cfg *config.Config, value int) string {
			cfg.MaxMatchGroups = value
			return fmt.Sprintf("%d", cfg.MaxMatchGroups)
		})
	case "7":
		return a.updateReasoningBudgetSetting(reader, stdout, stderr)
	case "8":
		return a.updateBooleanSetting(reader, stdout, stderr, "aggressive prepare rewrites", a.config.Advanced.AggressivePrepareRewrites, "settings: aggressive prepare rewrites unchanged", func(cfg *config.Config, value bool) string {
			cfg.Advanced.AggressivePrepareRewrites = value
			return enabledLabel(cfg.Advanced.AggressivePrepareRewrites)
		})
	case "9":
		return a.updateBooleanSetting(reader, stdout, stderr, "noise prefiltering", a.config.Advanced.NoisePrefiltering, "settings: noise prefiltering unchanged", func(cfg *config.Config, value bool) string {
			cfg.Advanced.NoisePrefiltering = value
			return enabledLabel(cfg.Advanced.NoisePrefiltering)
		})
	case "10":
		return a.updateBooleanSetting(reader, stdout, stderr, "adaptive budgets", a.config.Advanced.AdaptiveBudgets, "settings: adaptive budgets unchanged", func(cfg *config.Config, value bool) string {
			cfg.Advanced.AdaptiveBudgets = value
			return enabledLabel(cfg.Advanced.AdaptiveBudgets)
		})
	case "11":
		return a.updateBooleanSetting(reader, stdout, stderr, "early capture stop", a.config.Advanced.EarlyCaptureStop, "settings: early capture stop unchanged", func(cfg *config.Config, value bool) string {
			cfg.Advanced.EarlyCaptureStop = value
			return enabledLabel(cfg.Advanced.EarlyCaptureStop)
		})
	case "12":
		return a.updateBooleanSetting(reader, stdout, stderr, "semantic compaction", a.config.Advanced.SemanticCompaction, "settings: semantic compaction unchanged", func(cfg *config.Config, value bool) string {
			cfg.Advanced.SemanticCompaction = value
			return enabledLabel(cfg.Advanced.SemanticCompaction)
		})
	case "13":
		return a.updateBooleanSetting(reader, stdout, stderr, "compression contract", a.config.Advanced.CompressionContract, "settings: compression contract unchanged", func(cfg *config.Config, value bool) string {
			cfg.Advanced.CompressionContract = value
			return enabledLabel(cfg.Advanced.CompressionContract)
		})
	case "14":
		return a.updateBooleanSetting(reader, stdout, stderr, "compact artifact refs", a.config.Advanced.CompactArtifactRefs, "settings: compact artifact refs unchanged", func(cfg *config.Config, value bool) string {
			cfg.Advanced.CompactArtifactRefs = value
			return enabledLabel(cfg.Advanced.CompactArtifactRefs)
		})
	case "q", "quit", "exit":
		fmt.Fprintln(stdout, "settings: saved and exiting")
		return 0, true
	default:
		fmt.Fprintf(stdout, "settings: unknown choice %q\n\n", choice)
		return 0, false
	}
}

func (a *App) updateBooleanSetting(reader *bufio.Reader, stdout, stderr io.Writer, promptLabel string, current bool, unchangedMessage string, apply func(*config.Config, bool) string) (int, bool) {
	value, ok := promptForBooleanChoice(reader, stdout, promptLabel, current)
	if !ok {
		fmt.Fprintln(stdout, unchangedMessage)
		fmt.Fprintln(stdout)
		return 0, false
	}
	return a.saveSettings(stdout, stderr, promptLabel, func(cfg *config.Config) string {
		return apply(cfg, value)
	})
}

func (a *App) saveSettings(stdout, stderr io.Writer, label string, mutate func(*config.Config) string) (int, bool) {
	next := a.config
	value := mutate(&next)
	if err := a.persistConfig(next); err != nil {
		fmt.Fprintf(stderr, "szr: failed to save settings: %v\n", err)
		return 1, true
	}
	fmt.Fprintf(stdout, "saved: %s %s\n\n", label, value)
	return 0, false
}

func (a *App) updatePositiveIntSetting(reader *bufio.Reader, stdout, stderr io.Writer, promptLabel, savedLabel, unchangedMessage string, apply func(*config.Config, int) string) (int, bool) {
	value, ok := promptForPositiveInt(reader, stdout, promptLabel)
	if !ok {
		fmt.Fprintln(stdout, unchangedMessage)
		fmt.Fprintln(stdout)
		return 0, false
	}
	return a.saveSettings(stdout, stderr, savedLabel, func(cfg *config.Config) string {
		return apply(cfg, value)
	})
}

func (a *App) updateReasoningBudgetSetting(reader *bufio.Reader, stdout, stderr io.Writer) (int, bool) {
	mode, ok := promptForReasoningBudget(reader, stdout, a.config.ReasoningBudgetMode)
	if !ok {
		fmt.Fprintln(stdout, "settings: reasoning budget unchanged")
		fmt.Fprintln(stdout)
		return 0, false
	}
	return a.saveSettings(stdout, stderr, "reasoning budget mode", func(cfg *config.Config) string {
		cfg.ReasoningBudgetMode = mode
		return cfg.ReasoningBudgetMode
	})
}

func (a *App) persistConfig(cfg config.Config) error {
	if err := config.Save(a.paths, cfg); err != nil {
		return err
	}
	cfg, err := config.Normalize(cfg)
	if err != nil {
		return err
	}
	a.config = cfg
	a.engine = engine.New(cfg, a.paths, a.history, profiles.Builtins(cfg.MaxPreviewLines))
	return nil
}

func (a *App) printSettingsMenu(w io.Writer, cfg config.Config, configFile string) {
	ui := spreadUI{color: shouldColorizeStdout()}
	a.printMenuHeader(ui, "SZR SETTINGS", "interactive configuration")
	fmt.Fprintf(w, "config: %s\n", configFile)
	fmt.Fprintln(w, strings.Repeat("=", 54))
	printSettingsRow(w, "1", "update checks", enabledLabel(cfg.UpdateCheck.Enabled))
	printSettingsRow(w, "2", "auto update", enabledLabel(cfg.UpdateCheck.AutoUpdate))
	printSettingsRow(w, "3", "update interval hours", fmt.Sprintf("%d", cfg.UpdateCheck.IntervalHours))
	printSettingsRow(w, "4", "tee on failure", enabledLabel(cfg.TeeOnFailure))
	printSettingsRow(w, "5", "max preview lines", fmt.Sprintf("%d", cfg.MaxPreviewLines))
	printSettingsRow(w, "6", "max match groups", fmt.Sprintf("%d", cfg.MaxMatchGroups))
	printSettingsRow(w, "7", "reasoning budget mode", cfg.ReasoningBudgetMode+" "+dimSettingsTag("default"))
	printSettingsRow(w, "8", "aggressive prepare rewrites", enabledLabel(cfg.Advanced.AggressivePrepareRewrites))
	printSettingsRow(w, "9", "noise prefiltering", enabledLabel(cfg.Advanced.NoisePrefiltering))
	printSettingsRow(w, "10", "adaptive budgets", enabledLabel(cfg.Advanced.AdaptiveBudgets))
	printSettingsRow(w, "11", "early capture stop", enabledLabel(cfg.Advanced.EarlyCaptureStop))
	printSettingsRow(w, "12", "semantic compaction", enabledLabel(cfg.Advanced.SemanticCompaction))
	printSettingsRow(w, "13", "compression contract", enabledLabel(cfg.Advanced.CompressionContract))
	printSettingsRow(w, "14", "compact artifact refs", enabledLabel(cfg.Advanced.CompactArtifactRefs))
	fmt.Fprintln(w, strings.Repeat("-", 54))
	printSettingsRow(w, "q", "save and exit", "")
	fmt.Fprint(w, "> ")
}

func printSettingsRow(w io.Writer, key, label, value string) {
	const labelWidth = 28
	left := fmt.Sprintf("%s. %s", key, label)
	padding := labelWidth - utf8.RuneCountInString(left)
	if padding < 1 {
		padding = 1
	}
	if value == "" {
		fmt.Fprintf(w, "%s\n", left)
		return
	}
	fmt.Fprintf(w, "%s%s| [%s]\n", left, strings.Repeat(" ", padding), value)
}

func promptForPositiveInt(reader *bufio.Reader, stdout io.Writer, label string) (int, bool) {
	for {
		fmt.Fprintf(stdout, "%s\n", label)
		fmt.Fprintln(stdout, "  enter a positive integer")
		fmt.Fprintln(stdout, "  blank input cancels")
		fmt.Fprint(stdout, "> ")
		line, ok, err := readSettingsLine(reader)
		if err != nil || !ok {
			return 0, false
		}
		if line == "" {
			return 0, false
		}
		value, err := strconv.Atoi(line)
		if err != nil || value <= 0 {
			fmt.Fprintln(stdout, "enter a positive integer")
			continue
		}
		return value, true
	}
}

func promptForBooleanChoice(reader *bufio.Reader, stdout io.Writer, label string, current bool) (bool, bool) {
	for {
		fmt.Fprintf(stdout, "%s\n", label)
		fmt.Fprintf(stdout, "  current: %s\n", enabledLabel(current))
		fmt.Fprintln(stdout, "  1. enable")
		fmt.Fprintln(stdout, "  2. disable")
		fmt.Fprintln(stdout, "  blank input cancels")
		fmt.Fprint(stdout, "> ")
		line, ok, err := readSettingsLine(reader)
		if err != nil || !ok {
			return false, false
		}
		switch strings.ToLower(line) {
		case "":
			return false, false
		case "1", "enable", "enabled", "on", "true":
			return true, true
		case "2", "disable", "disabled", "off", "false":
			return false, true
		default:
			fmt.Fprintln(stdout, "choose 1 to enable or 2 to disable")
		}
	}
}

func promptForReasoningBudget(reader *bufio.Reader, stdout io.Writer, current string) (string, bool) {
	for {
		fmt.Fprintln(stdout, "reasoning budget mode")
		fmt.Fprintf(stdout, "  current: %s\n", current)
		fmt.Fprintf(stdout, "  1. standard %s %s - balanced for human readability\n", dimSettingsTag("default"), dimSettingsTag("recommended"))
		fmt.Fprintln(stdout, "  2. agent     - steadier for agent loops")
		fmt.Fprintln(stdout, "  3. aggressive - tighter for spread-heavy workflows")
		fmt.Fprintln(stdout, "  blank input cancels")
		fmt.Fprint(stdout, "> ")
		line, ok, err := readSettingsLine(reader)
		if err != nil || !ok {
			return "", false
		}
		switch strings.ToLower(line) {
		case "":
			return "", false
		case "1":
			line = config.ReasoningBudgetStandard
		case "2":
			line = config.ReasoningBudgetAgent
		case "3":
			line = config.ReasoningBudgetAggressive
		}
		mode, err := config.NormalizeReasoningBudgetMode(line)
		if err != nil {
			fmt.Fprintln(stdout, err.Error())
			continue
		}
		return mode, true
	}
}

func readSettingsLine(reader *bufio.Reader) (string, bool, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			line = strings.TrimSpace(line)
			if line == "" {
				return "", false, nil
			}
			return line, true, nil
		}
		return "", false, err
	}
	return strings.TrimSpace(line), true, nil
}

func enabledLabel(value bool) string {
	if value {
		return "enabled"
	}
	return "disabled"
}

func dimSettingsTag(label string) string {
	tag := "[" + label + "]"
	if shouldColorizeStdout() {
		return ansiDim + tag + ansiReset
	}
	return tag
}
