#!/usr/bin/env python3
"""Area 7 (reports): gremlins only mutates Go operators, and the reports live in SQL. This plants one SQL / Go
mistake at a time in a SCRATCH copy and runs the area's tests; every one must be KILLED.
  rsync -a --delete --exclude .git --exclude web/node_modules ./ ~/.cache/karots-mutant/ && cd ~/.cache/karots-mutant
  go build -o bin/testdb ./scripts/testdb && python3 scripts/report-mutants.py . ["name,name"]"""
import os, subprocess, sys

root = os.path.abspath(sys.argv[1])
R = 'internal/features/reports/'
M = [
    ("returns on the sale's day", R+'flow.go', "WHERE NOT p.pass_through AND s.status <> 'void' AND r.created_at >= $1 AND r.created_at < $2",
     "WHERE NOT p.pass_through AND s.status <> 'void' AND s.created_at >= $1 AND s.created_at < $2"),
    ('P&L returns from sold rows', R+'reports.go', "COALESCE(-SUM(revenue) FILTER (WHERE kind = 'r'),0) AS returns", "COALESCE(-SUM(revenue) FILTER (WHERE kind = 's'),0) AS returns"),
    ('discount shared onto pass-through', R+'flow.go', "WHERE x.sale_id = s.id AND NOT xp.pass_through), 0), 0)", "WHERE x.sale_id = s.id), 0), 0)"),
    ('pass-through face left in the factor', R+'flow.go', "WHERE x.sale_id = s.id AND xp.pass_through)) / NULLIF", "WHERE x.sale_id = s.id AND false)) / NULLIF"),
    ('refunds with tax in', R+'flow.go', "refundPreTaxSQL = `COALESCE((s.total - s.tax - ", "refundPreTaxSQL = `COALESCE((s.total - 0 - "),
    ('no tax given back', R+'reports_ops.go', "COALESCE(SUM(x.back),0) AS given_back", "0 AS given_back"),
    ('pass-through lines as revenue', R+'flow.go', "FROM sale_items si JOIN sales s ON s.id = si.sale_id JOIN products p ON p.id = si.product_id\n\tWHERE NOT p.pass_through AND",
     "FROM sale_items si JOIN sales s ON s.id = si.sale_id JOIN products p ON p.id = si.product_id\n\tWHERE true AND"),
    ('void sales in the flow', R+'flow.go', "WHERE NOT p.pass_through AND s.status <> 'void' AND s.created_at >= $1", "WHERE NOT p.pass_through AND s.created_at >= $1"),
    ('tax in the trend', R+'flow.go', "\t\ts.total - s.tax - (SELECT", "\t\ts.total - (SELECT"),
    ('void counted in the trend', R+'flow.go', "FROM sales s WHERE s.status <> 'void' AND s.created_at", "FROM sales s WHERE s.created_at"),
    ('no-cost lines count returns', R+'reports.go', "COUNT(*) FILTER (WHERE no_cost AND kind = 's')", "COUNT(*) FILTER (WHERE no_cost)"),
    ('opening stock as profit', R+'reports.go', "WHERE type = 'adjust' AND reference_type IS DISTINCT FROM 'opening' AND", "WHERE type = 'adjust' AND"),
    ('first stock by AddStock is found', 'internal/features/stock/add.go', 'if in.Source == "opening" || in.OnBooks || first {', 'if in.Source == "opening" || in.OnBooks || (first && false) {'),
    ('+ Add batch is found', 'internal/features/stock/add.go', 'if in.Source == "opening" || in.OnBooks || first {', 'if in.Source == "opening" || first {'),
    ('first count is found', 'internal/features/stock/service.go', '\t\t\tif first {\n', '\t\t\tif first && false {\n'),
    ('Z by the sale session', 'internal/features/sales/service_summary.go', 'WHERE r.session_id = $1 AND', 'WHERE s.session_id = $1 AND'),
    ('cashiers without returns', R+'reports_sales.go', "FROM (`+soldSQL+` UNION ALL `+returnedSQL+`) x\n\t\t) x JOIN users", "FROM (`+soldSQL+`) x\n\t\t) x JOIN users"),
    ('Sales report with tax in', 'internal/features/sales/store_query.go', 'const netRevSQL = `(s.total - s.tax - agg.pt_face', 'const netRevSQL = `(s.total - agg.pt_face'),
    ('Sales report counts void', 'internal/features/sales/store_query.go', "(($4::text IS NULL AND s.status <> 'void') OR", "(($4::text IS NULL) OR"),
    ('Sales report refunds at pass-through', 'internal/features/sales/store_query.go', 'WHERE r.sale_id = s.id AND NOT rp.pass_through), 0) AS ret_val', 'WHERE r.sale_id = s.id AND rp.pass_through), 0) AS ret_val'),
    ('a weaker snapshot', R+'flow.go', 'Isolation: sql.LevelRepeatableRead', 'Isolation: sql.LevelReadCommitted'),
]
run = 'TestReportsAgreeOverAShopDay|TestRPT|TestZR|TestBattleSales'
env = dict(os.environ)
testdb = os.path.join(root, 'bin/testdb')
only = sys.argv[2].split(',') if len(sys.argv) > 2 else None
for name, f, old, new in M:
    if only and name not in only:
        continue
    p = os.path.join(root, f)
    src = open(p).read()
    if src.count(old) != 1:
        print(f'NOT PLANTED  {name}: found {src.count(old)}×', flush=True)
        continue
    open(p, 'w').write(src.replace(old, new))
    try:
        r = subprocess.run(['go', 'test', '-exec', testdb + ' exec', './internal/battle', '-run', run, '-count=1'],
                           cwd=root, env=env, capture_output=True, text=True, timeout=900)
        out = r.stdout + r.stderr
        if r.returncode == 0:
            print(f'LIVED   {name}', flush=True)
        else:
            why = next((l.strip() for l in out.splitlines() if '_test.go:' in l), out.strip().splitlines()[-1] if out.strip() else '?')
            print(f'KILLED  {name}  ← {why[:150]}', flush=True)
    finally:
        open(p, 'w').write(src)
