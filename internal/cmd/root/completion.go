package root

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/output"
)

const (
	completionBlockStart = "# >>> life-ustc completion >>>"
	completionBlockEnd   = "# <<< life-ustc completion <<<"
)

func newCmdCompletion() *cobra.Command {
	var shell string

	cmd := &cobra.Command{
		Use:   "completion [shell]",
		Short: "Generate or install shell completions",
		Long:  `Generate a completion script or install it into your shell profile.`,
		Example: `  # Print a completion script to stdout
  life-ustc completion -s zsh

  # Install completion for the current shell
  life-ustc completion install

  # Install completion for bash explicitly
  life-ustc completion install -s bash`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			shellName, err := resolveShell(shell, args)
			if err != nil {
				return err
			}
			return writeCompletion(cmd.Root(), shellName, os.Stdout)
		},
		ValidArgsFunction: completeShells,
	}

	cmd.Flags().StringVarP(&shell, "shell", "s", "", "Shell type: bash, zsh, fish, powershell")
	_ = cmd.RegisterFlagCompletionFunc("shell", completeShells)
	cmd.AddCommand(newCmdCompletionInstall())

	return cmd
}

func newCmdCompletionInstall() *cobra.Command {
	var shell string

	cmd := &cobra.Command{
		Use:   "install [shell]",
		Short: "Install shell completions into your shell profile",
		Long: `Install shell completion for your current user without requiring manual registration.

The command writes the generated completion script to the shell's user-local
completion directory and updates the appropriate shell startup file when the
shell needs an explicit source or fpath entry.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			shellName, err := resolveShell(shell, args)
			if err != nil {
				return err
			}

			install, err := planCompletionInstall(shellName)
			if err != nil {
				return err
			}

			var script bytes.Buffer
			if err := writeCompletion(cmd.Root(), shellName, &script); err != nil {
				return err
			}

			actualPath, err := install.tryInstall(script.Bytes())
			if err != nil {
				return err
			}

			if install.fallbackUsed && install.rcPath != "" && install.rcBlock != "" {
				if err := upsertManagedBlock(install.rcPath, install.rcBlock); err != nil {
					return err
				}
			}

			output.Success(fmt.Sprintf("Installed %s completion.", shellName))
			output.Info(fmt.Sprintf("Script: %s", actualPath))
			if install.fallbackUsed && install.rcPath != "" && install.rcBlock != "" {
				output.Info(fmt.Sprintf("Shell profile: %s", install.rcPath))
			}
			if install.reloadHint != "" {
				if install.fallbackUsed {
					output.Hint(fmt.Sprintf("restart your shell or run: source %s", install.rcPath))
				} else {
					output.Hint(install.reloadHint)
				}
			}
			return nil
		},
		ValidArgsFunction: completeShells,
	}

	cmd.Flags().StringVarP(&shell, "shell", "s", "", "Shell type: bash, zsh, fish, powershell")
	_ = cmd.RegisterFlagCompletionFunc("shell", completeShells)

	return cmd
}

func writeCompletion(root *cobra.Command, shell string, w io.Writer) error {
	switch shell {
	case "bash":
		var buf bytes.Buffer
		if err := root.GenBashCompletion(&buf); err != nil {
			return err
		}
		_, err := io.WriteString(w, patchBashCompletion(buf.String()))
		return err
	case "zsh":
		return root.GenZshCompletion(w)
	case "fish":
		return root.GenFishCompletion(w, true)
	case "powershell":
		return root.GenPowerShellCompletionWithDesc(w)
	default:
		return fmt.Errorf("unsupported shell %q (use bash, zsh, fish, or powershell)", shell)
	}
}

func patchBashCompletion(script string) string {
	return script + bashCompletionDescriptionPatch
}

const bashCompletionDescriptionPatch = `

# life-ustc enhancement: keep completion descriptions and print them in bash.

__life-ustc_request_completion()
{
    local requestComp lastParam lastChar args

    args=("${words[@]:1}")
    requestComp="LIFE_USTC_ACTIVE_HELP=0 ${words[0]} __complete ${args[*]}"

    lastParam=${words[$((${#words[@]}-1))]}
    lastChar=${lastParam:$((${#lastParam}-1)):1}
    if [ -z "${cur}" ] && [ "${lastChar}" != "=" ]; then
        requestComp="${requestComp} \"\""
    fi

    __life_ustc_completion_out=$(eval "${requestComp}" 2>/dev/null)
    __life_ustc_completion_directive=${__life_ustc_completion_out##*:}
    __life_ustc_completion_out=${__life_ustc_completion_out%:*}
    if [ "${__life_ustc_completion_directive}" = "${__life_ustc_completion_out}" ]; then
        __life_ustc_completion_directive=0
    fi
}

__life-ustc_print_completion_descriptions()
{
    if [[ "${LIFE_USTC_BASH_COMP_DESCRIPTIONS:-1}" = "0" ]] || [[ ! -t 2 ]] || [[ "${cur}" = -* ]] || [[ ${#COMPREPLY[@]} -eq 0 ]]; then
        return
    fi

    local out directive line choice desc
    local -a shownChoices shownDescs

    __life-ustc_request_completion
    out=${__life_ustc_completion_out}
    directive=${__life_ustc_completion_directive}
    if [ $((directive & 1)) -ne 0 ]; then
        return
    fi

    while IFS='' read -r line; do
        [ -z "$line" ] && continue
        choice=${line%%$'\t'*}
        desc=""
        if [[ "$line" == *$'\t'* ]]; then
            desc=${line#*$'\t'}
        fi
        [ -z "$desc" ] && continue
        for comp in "${COMPREPLY[@]}"; do
            if [ "$comp" = "$choice" ]; then
                shownChoices+=("$choice")
                shownDescs+=("$desc")
                break
            fi
        done
    done <<< "${out}"

    if [ ${#shownChoices[@]} -eq 0 ]; then
        return
    fi

    local maxWidth=0 i
    for choice in "${shownChoices[@]}"; do
        if [ ${#choice} -gt $maxWidth ]; then
            maxWidth=${#choice}
        fi
    done

    printf '\n' >&2
    for i in "${!shownChoices[@]}"; do
        choice=${shownChoices[$i]}
        desc=${shownDescs[$i]}
        printf '  \033[1m%s\033[0m%*s  \033[2m%s\033[0m\n' "$choice" "$((maxWidth-${#choice}))" "" "$desc" >&2
    done
}

__life-ustc_handle_go_custom_completion()
{
    __life-ustc_debug "${FUNCNAME[0]}: cur is ${cur}, words[*] is ${words[*]}, #words[@] is ${#words[@]}"

    local shellCompDirectiveError=1
    local shellCompDirectiveNoSpace=2
    local shellCompDirectiveNoFileComp=4
    local shellCompDirectiveFilterFileExt=8
    local shellCompDirectiveFilterDirs=16

    local out directive line comp
    local -a completions

    __life-ustc_request_completion
    out=${__life_ustc_completion_out}
    directive=${__life_ustc_completion_directive}

    if [ $((directive & shellCompDirectiveError)) -ne 0 ]; then
        return
    fi

    if [ $((directive & shellCompDirectiveNoSpace)) -ne 0 ] && [[ $(type -t compopt) = "builtin" ]]; then
        compopt -o nospace
    fi
    if [ $((directive & shellCompDirectiveNoFileComp)) -ne 0 ] && [[ $(type -t compopt) = "builtin" ]]; then
        compopt +o default
    fi

    if [ $((directive & shellCompDirectiveFilterFileExt)) -ne 0 ]; then
        local fullFilter filter
        for filter in ${out}; do
            fullFilter+="$filter|"
        done
        _filedir "$fullFilter"
        return
    fi
    if [ $((directive & shellCompDirectiveFilterDirs)) -ne 0 ]; then
        local subdir
        subdir=$(printf "%s" "${out}")
        if [ -n "$subdir" ]; then
            __life-ustc_handle_subdirs_in_dir_flag "$subdir"
        else
            _filedir -d
        fi
        return
    fi

    while IFS='' read -r line; do
        [ -z "$line" ] && continue
        comp=${line%%$'\t'*}
        completions+=("$comp")
    done <<< "${out}"

    COMPREPLY=()
    for comp in "${completions[@]}"; do
        if [[ "$comp" == "$cur"* ]]; then
            COMPREPLY+=("$comp")
        fi
    done

}

if declare -F __life-ustc_handle_reply >/dev/null; then
    eval "$(declare -f __life-ustc_handle_reply | sed '1s/__life-ustc_handle_reply/__life-ustc_handle_reply_original/')"
    __life-ustc_handle_reply()
    {
        __life-ustc_handle_reply_original "$@"
        __life-ustc_print_completion_descriptions
    }
fi
`

func resolveShell(shellFlag string, args []string) (string, error) {
	if shellFlag != "" {
		return normalizeShell(shellFlag)
	}
	if len(args) > 0 {
		return normalizeShell(args[0])
	}
	shell, ok := detectShell()
	if !ok {
		return "", fmt.Errorf("could not detect your shell; pass --shell")
	}
	return shell, nil
}

func detectShell() (string, bool) {
	if shell := os.Getenv("SHELL"); shell != "" {
		if normalized, err := normalizeShell(filepath.Base(shell)); err == nil {
			return normalized, true
		}
	}
	if runtime.GOOS == "windows" {
		return "powershell", true
	}
	return "", false
}

func normalizeShell(shell string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(shell)) {
	case "bash":
		return "bash", nil
	case "zsh":
		return "zsh", nil
	case "fish":
		return "fish", nil
	case "powershell", "pwsh", "ps":
		return "powershell", nil
	default:
		return "", fmt.Errorf("unsupported shell %q", shell)
	}
}

func completeShells(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	completions := []string{
		"bash\tGNU Bash",
		"zsh\tZ shell",
		"fish\tFish shell",
		"powershell\tPowerShell",
	}
	return filterCompletions(completions, toComplete, "Use --shell with bash, zsh, fish, or powershell."), cobra.ShellCompDirectiveNoFileComp
}

type completionInstall struct {
	scriptPath         string
	fallbackScriptPath string
	rcPath             string
	rcBlock            string
	reloadHint         string
	fallbackUsed       bool
}

func (ci *completionInstall) tryInstall(script []byte) (string, error) {
	if err := os.MkdirAll(filepath.Dir(ci.scriptPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(ci.scriptPath, script, 0o644); err != nil {
		if ci.fallbackScriptPath != "" && os.IsPermission(err) {
			return ci.tryFallback(script)
		}
		return "", err
	}
	return ci.scriptPath, nil
}

func (ci *completionInstall) tryFallback(script []byte) (string, error) {
	if err := os.MkdirAll(filepath.Dir(ci.fallbackScriptPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(ci.fallbackScriptPath, script, 0o644); err != nil {
		return "", err
	}
	ci.fallbackUsed = true
	return ci.fallbackScriptPath, nil
}

func planCompletionInstall(shell string) (*completionInstall, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	switch shell {
	case "bash":
		return planBashInstall(home), nil
	case "zsh":
		return planZshInstall(home), nil
	case "fish":
		configHome := configHome(home)
		fishConfig := filepath.Join(configHome, "fish")
		return &completionInstall{
			scriptPath: filepath.Join(fishConfig, "completions", "life-ustc.fish"),
			reloadHint: fmt.Sprintf("restart fish or run: source %s", filepath.Join(fishConfig, "config.fish")),
		}, nil
	case "powershell":
		scriptPath, profilePath := powerShellPaths(home)
		return &completionInstall{
			scriptPath: scriptPath,
			rcPath:     profilePath,
			rcBlock:    managedBlock(fmt.Sprintf(`. %q`, scriptPath)),
			reloadHint: fmt.Sprintf("restart PowerShell or run: . %q", profilePath),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported shell %q", shell)
	}
}

func planZshInstall(home string) *completionInstall {
	fb := filepath.Join(home, ".zsh", "completions")
	fbScript := filepath.Join(fb, "_life-ustc")
	fbRCPath := filepath.Join(home, ".zshrc")
	fbRCBlock := managedBlock(fmt.Sprintf(`fpath=(%q $fpath)
if ! (( $+functions[compdef] )); then
  autoload -Uz compinit
  compinit -i
fi`, fb))

	for _, dir := range []string{
		"/usr/local/share/zsh/site-functions",
		"/usr/share/zsh/site-functions",
	} {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return &completionInstall{
				scriptPath:         filepath.Join(dir, "_life-ustc"),
				fallbackScriptPath: fbScript,
				rcPath:             fbRCPath,
				rcBlock:            fbRCBlock,
				reloadHint:         "restart your shell",
			}
		}
	}

	return &completionInstall{
		scriptPath: fbScript,
		rcPath:     fbRCPath,
		rcBlock:    fbRCBlock,
		reloadHint: fmt.Sprintf("restart your shell or run: source %s", fbRCPath),
	}
}

func planBashInstall(home string) *completionInstall {
	systemScript := "/usr/share/bash-completion/completions/life-ustc"
	userScript := filepath.Join(home, ".local", "share", "bash-completion", "completions", "life-ustc")
	rcPath := preferredBashRC(home)
	rcBlock := managedBlock(fmt.Sprintf(`for __life_ustc_bash_completion in \
  /usr/share/bash-completion/bash_completion \
  /etc/bash_completion; do
  if [ -f "$__life_ustc_bash_completion" ]; then
    source "$__life_ustc_bash_completion"
    break
  fi
done
unset __life_ustc_bash_completion
if [ -f %q ]; then
  source %q
fi`, userScript, userScript))

	if info, err := os.Stat(filepath.Dir(systemScript)); err == nil && info.IsDir() {
		return &completionInstall{
			scriptPath:         systemScript,
			fallbackScriptPath: userScript,
			rcPath:             rcPath,
			rcBlock:            rcBlock,
			reloadHint:         "restart your shell",
		}
	}

	return &completionInstall{
		scriptPath: userScript,
		rcPath:     rcPath,
		rcBlock:    rcBlock,
		reloadHint: fmt.Sprintf("restart your shell or run: source %s", rcPath),
	}
}

func preferredBashRC(home string) string {
	if runtime.GOOS == "darwin" {
		for _, name := range []string{".bash_profile", ".bashrc", ".profile"} {
			path := filepath.Join(home, name)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
		return filepath.Join(home, ".bash_profile")
	}

	for _, name := range []string{".bashrc", ".bash_profile", ".profile"} {
		path := filepath.Join(home, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return filepath.Join(home, ".bashrc")
}

func configHome(home string) string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg
	}
	return filepath.Join(home, ".config")
}

func powerShellPaths(home string) (string, string) {
	if runtime.GOOS == "windows" {
		documents := filepath.Join(home, "Documents", "PowerShell")
		return filepath.Join(documents, "Completions", "life-ustc.ps1"), filepath.Join(documents, "Microsoft.PowerShell_profile.ps1")
	}
	configDir := filepath.Join(configHome(home), "powershell")
	return filepath.Join(configDir, "Completions", "life-ustc.ps1"), filepath.Join(configDir, "Microsoft.PowerShell_profile.ps1")
}

func managedBlock(body string) string {
	return completionBlockStart + "\n" + body + "\n" + completionBlockEnd + "\n"
}

func upsertManagedBlock(path, block string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	content := string(existing)
	start := strings.Index(content, completionBlockStart)
	end := strings.Index(content, completionBlockEnd)
	if start >= 0 && end >= start {
		end += len(completionBlockEnd)
		if end < len(content) && content[end] == '\n' {
			end++
		}
		content = strings.TrimRight(content[:start]+content[end:], "\n")
	}

	if content != "" {
		content += "\n\n"
	}
	content += block

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
