#!/usr/bin/env python3
"""AWS を変更する操作を、Claude が実行しないように止める PreToolUse フック (Bash)

止めるもの
    - terraform: apply / destroy / import / taint / untaint / force-unlock、state の rm / mv / push / replace-provider
    - just: apply / destroy-target / allow-my-ip / push のレシピ (infra:: や backend:: を付けた形も)
    - scripts/allow-my-ip.sh、scripts/push.sh の直接の実行
    - docker push
    - aws CLI のうち、読み取り (describe / list / get など) 以外の操作

通すもの
    - terraform の init / plan / fmt / validate / output / show / state list など、tflint、trivy
    - aws CLI の読み取り

コマンドは ; && || | ( ) 改行 で区切り、区切りごとに先頭のコマンドを見る
    - heredoc の本文と、クォートの中の文字は区切りとして扱わない (文章の中の語を、コマンドと取り違えないため)
    - bash -c / sh -c に渡した文字列は、中身も同じように調べる
"""

import json
import re
import shlex
import sys

TERRAFORM_DENY = {"apply", "destroy", "import", "taint", "untaint", "force-unlock"}
TERRAFORM_STATE_DENY = {"rm", "mv", "push", "replace-provider"}
JUST_DENY = {"apply", "destroy-target", "allow-my-ip", "push"}
SCRIPT_DENY = {"allow-my-ip.sh", "push.sh"}

# aws CLI で通す操作 (読み取りだけ)
AWS_ALLOW_PREFIXES = ("describe-", "list-", "get-", "head-", "batch-get-", "lookup-", "search-", "filter-")
AWS_ALLOW_EXACT = {"help", "wait", "tail", "ls"}
# サービスごとに通す、上の決まりに当てはまらない読み取りの操作
AWS_ALLOW_BY_SERVICE = {"dynamodb": {"scan", "query"}}
# 値を取らない aws のグローバルオプション
AWS_BOOL_OPTIONS = {"--debug", "--no-verify-ssl", "--no-paginate", "--no-sign-request", "--no-cli-pager", "--version"}

PREFIX_COMMANDS = {"env", "sudo", "time", "nohup", "command", "exec", "xargs"}
SEPARATORS = set(";&|()")
HEREDOC = re.compile(r"<<-?\s*(['\"]?)(\w+)\1[^\n]*\n.*?\n\s*\2\s*(?=\n|$)", re.S)


def strip_heredocs(command):
    """heredoc の本文を取り除く (開始の行の残りは残す)"""
    return HEREDOC.sub(lambda m: m.group(0).split("\n", 1)[0].split("<<", 1)[0], command)


def newlines_to_separators(command):
    """クォートの外にある改行だけを ; に置き換える (クォートの中の改行は文字列の一部のため残す)"""
    out = []
    quote = None
    escaped = False
    for ch in command:
        if escaped:
            escaped = False
        elif ch == "\\" and quote != "'":
            escaped = True
        elif quote:
            if ch == quote:
                quote = None
        elif ch in ("'", '"'):
            quote = ch
        elif ch == "\n":
            ch = ";"
        out.append(ch)
    return "".join(out)


def split_segments(command):
    """クォートを考えて字句に分け、区切りの記号ごとに字句の並びを返す"""
    text = newlines_to_separators(strip_heredocs(command))
    lexer = shlex.shlex(text, posix=True, punctuation_chars=";&|()")
    lexer.whitespace_split = True
    try:
        tokens = list(lexer)
    except ValueError:
        tokens = text.split()

    segments = []
    current = []
    for tok in tokens:
        if tok and set(tok) <= SEPARATORS:
            segments.append(current)
            current = []
            continue
        current.append(tok)
    segments.append(current)
    return [s for s in segments if s]


def strip_prefix(tokens):
    """環境変数の代入 (A=b) と、env や sudo などの前置きを外す"""
    i = 0
    while i < len(tokens):
        tok = tokens[i]
        if re.match(r"^[A-Za-z_][A-Za-z0-9_]*=", tok) or tok in PREFIX_COMMANDS:
            i += 1
            continue
        break
    return tokens[i:]


def non_flags(tokens):
    return [t for t in tokens if not t.startswith("-")]


def check_terraform(args):
    words = non_flags(args)
    if not words:
        return None
    if words[0] in TERRAFORM_DENY:
        return f"terraform {words[0]}"
    if words[0] == "state" and len(words) > 1 and words[1] in TERRAFORM_STATE_DENY:
        return f"terraform state {words[1]}"
    return None


def check_just(args):
    for word in non_flags(args):
        recipe = word.split("::")[-1]
        if recipe in JUST_DENY:
            return f"just {word}"
    return None


def check_aws(args):
    i = 0
    while i < len(args) and args[i].startswith("-"):
        opt = args[i]
        i += 1
        if "=" not in opt and opt not in AWS_BOOL_OPTIONS and i < len(args) and not args[i].startswith("-"):
            i += 1
    rest = args[i:]
    if len(rest) < 2:
        return None
    service, operation = rest[0], rest[1]
    if operation in AWS_ALLOW_EXACT or operation.startswith(AWS_ALLOW_PREFIXES):
        return None
    if operation in AWS_ALLOW_BY_SERVICE.get(service, set()):
        return None
    if service == "ecr" and operation == "get-login-password":
        return None
    if service == "sts" and operation.startswith("get-"):
        return None
    return f"aws {service} {operation}"


def check_segment(tokens, depth=0):
    tokens = strip_prefix(tokens)
    if not tokens:
        return None
    cmd = tokens[0].rsplit("/", 1)[-1]
    args = tokens[1:]

    if cmd in ("bash", "sh", "zsh") and "-c" in args:
        idx = args.index("-c")
        if idx + 1 < len(args) and depth < 3:
            return check_command(args[idx + 1], depth + 1)
    if cmd == "terraform":
        return check_terraform(args)
    if cmd == "just":
        return check_just(args)
    if cmd in SCRIPT_DENY:
        return tokens[0]
    if cmd == "docker" and "push" in args[:2]:
        return "docker push"
    if cmd == "aws":
        return check_aws(args)
    return None


def check_command(command, depth=0):
    for segment in split_segments(command):
        found = check_segment(segment, depth)
        if found:
            return found
    return None


def main():
    payload = json.load(sys.stdin)
    if payload.get("tool_name") != "Bash":
        return 0
    command = payload.get("tool_input", {}).get("command", "")
    found = check_command(command)
    if not found:
        return 0
    print(
        f"AWS を変更する操作は、ユーザーだけが実行する決まりです: {found}\n"
        "実行するコマンドをユーザーに伝え、実行を依頼してください (CLAUDE.md の禁止事項)。",
        file=sys.stderr,
    )
    return 2


if __name__ == "__main__":
    sys.exit(main())
