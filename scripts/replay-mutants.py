#!/usr/bin/env python3
"""Replay gremlins' surviving mutants against the battle tests (make mutate only runs a package's own tests).

usage (from a scratch copy of the repo, so the working tree is never touched):
  rsync -a --exclude .git --exclude web/node_modules ./ ~/.cache/karots-mutant/ && cd ~/.cache/karots-mutant
  python3 scripts/replay-mutants.py ~/.cache/karots-logs/gremlins-<pkg>.log internal/features/<pkg> . 'TestSTK|TestStock'
Prints KILLED (with the test that failed) / LIVED / NOT VIABLE per mutant. Run nothing else in the battle package meanwhile.
"""
import os, re, subprocess, sys

log, pkg, root, run = sys.argv[1:5]
root = os.path.abspath(root)
NEG = {'==': '!=', '!=': '==', '<': '>=', '>': '<=', '<=': '>', '>=': '<'}
BND = {'<': '<=', '<=': '<', '>': '>=', '>=': '>'}
ARI = {'+': '-', '-': '+', '*': '/', '/': '*', '%': '*'}
INC = {'++': '--', '--': '++'}

def mutate(line, col, kind):
    i = col - 1
    two, one = line[i:i+2], line[i:i+1]
    if kind == 'INVERT_NEGATIVES' and one == '-':
        return line[:i] + line[i+1:]
    table = {'CONDITIONALS_NEGATION': NEG, 'CONDITIONALS_BOUNDARY': BND, 'ARITHMETIC_BASE': ARI,
             'INCREMENT_DECREMENT': INC}.get(kind, {})
    for tok in (two, one):
        if tok in table:
            return line[:i] + table[tok] + line[i+len(tok):]
    return None

env = dict(os.environ)
seen = set()
for raw in open(log):
    m = re.match(r'\s*(LIVED|NOT COVERED)\s+(\w+)\s+at\s+(\S+):(\d+):(\d+)', raw)
    if not m:
        continue
    _, kind, f, ln, col = m.groups()
    key = (kind, f, ln, col)
    if key in seen:
        continue
    seen.add(key)
    path = os.path.join(root, pkg, f)
    src = open(path).read().split('\n')
    new = mutate(src[int(ln)-1], int(col), kind)
    tag = f'{kind} {f}:{ln}:{col}'
    if new is None:
        print('SKIPPED', tag, flush=True); continue
    orig = src[int(ln)-1]
    src[int(ln)-1] = new
    open(path, 'w').write('\n'.join(src))
    try:
        r = subprocess.run(['go', 'test', '-exec', os.path.join(root, 'bin/testdb') + ' exec', './internal/battle',
                            '-run', run, '-count=1', '-timeout', '10m'], cwd=root, env=env,
                           capture_output=True, text=True)
        out = r.stdout + r.stderr
        if r.returncode == 0:
            res = 'LIVED'
        elif 'build failed' in out or 'setup failed' in out:
            res = 'NOT VIABLE'
        else:
            res = 'KILLED'
        why = ''
        if res == 'KILLED':
            why = next((l.strip() for l in out.splitlines() if l.startswith('--- FAIL') or 'ltest.go' in l or '_test.go:' in l), '?')
        print(res, tag, '|', orig.strip()[:70], '|', why[:110], flush=True)
    finally:
        src[int(ln)-1] = orig
        open(path, 'w').write('\n'.join(src))
