import Karots

/-!
Writes `cases/refunds.json` (Karots.Refund) and `cases/shares.json` (Karots.Share)
from the proven models: the refund
for `n` more of a line paid `P` cents for `Q` thousandths when `R` were already
back. The Go server (`sales.PartRefund`) and the till (`refund.ts`) must give
each one to the cent. Seeded, so the file only changes when the model does.
-/

open Karots

/-- A small linear congruential generator: the same cases every run. -/
def next (s : Nat) : Nat := (s * 6364136223846793005 + 1442695040888963407) % 18446744073709551616

def gen : Nat → Nat → List String → List String
  | 0, _, acc => acc.reverse
  | k + 1, s, acc =>
    let s1 := next s; let s2 := next s1; let s3 := next s2; let s4 := next s3
    let P := s1 / 65536 % 5000000              -- up to Rs 50,000
    let Q := 1 + s2 / 65536 % 20000            -- up to 20 units, in thousandths
    let R := s3 / 65536 % Q                    -- already back
    let n := 1 + s4 / 65536 % (Q - R)          -- coming back now
    let line := s!"\{\"paid\":{P},\"qty\":{Q},\"before\":{R},\"back\":{n},\"refund\":{refund P Q R n}}"
    gen k s4 (line :: acc)

/-- Sharing cases: an amount over up to 8 lines' nets (cents), and each share. -/
def genShares : Nat → Nat → List String → List String
  | 0, _, acc => acc.reverse
  | k + 1, s, acc =>
    let s1 := next s
    let n := 1 + s1 / 65536 % 8
    let (ws, s2) := (List.range n).foldl (fun (l, t) _ => let t' := next t; (l ++ [t' / 65536 % 200000], t')) ([], s1)
    let s3 := next s2
    let a := if ws.sum = 0 then 0 else s3 / 65536 % (ws.sum + 1)
    let arr (l : List Nat) := "[" ++ ",".intercalate (l.map toString) ++ "]"
    let line := s!"\{\"amount\":{a},\"weights\":{arr ws},\"shares\":{arr (shareOut a ws)}}"
    genShares k s3 (line :: acc)

def write (file : String) (rows : List String) : IO Unit := do
  IO.FS.writeFile file ("[\n" ++ ",\n".intercalate rows ++ "\n]\n")
  IO.println s!"{file}: {rows.length} cases"

def main : IO Unit := do
  IO.FS.createDirAll "cases"
  write "cases/refunds.json" (gen 3000 20261118 [])
  write "cases/shares.json" (genShares 3000 20261118 [])
