import Karots

/-!
Writes `cases/refunds.json` from the proven model (Karots.Refund): the refund
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

def main : IO Unit := do
  IO.FS.createDirAll "cases"
  let rows := gen 3000 20261118 []
  IO.FS.writeFile "cases/refunds.json" ("[\n" ++ ",\n".intercalate rows ++ "\n]\n")
  IO.println s!"cases/refunds.json: {rows.length} cases"
