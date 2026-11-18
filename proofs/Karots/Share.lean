/-!
# Sharing an amount over lines (DISC-5, RET-5, SALE-35)

What was paid (or a bill discount) is shared over a sale's lines in
proportion to their nets `ws` (cents, `B = Σ ws`): each line first takes its
exact share rounded down, `w × a ÷ B`. The cents left over then go one each to
the lines with the largest remainders (`sales.shareOut`, `totals.ts shareOut`).

Proved here: the rounded-down shares never add up to more than the amount, and
fall short by fewer cents than there are lines — so giving the leftover cents
one per line is always possible, the shares add up to the amount exactly, and
no line is more than a cent from its exact share.
-/

namespace Karots

/-- Each line's share rounded down. -/
def floors (a B : Nat) (ws : List Nat) : List Nat := ws.map (fun w => w * a / B)

theorem add_div_le (x y B : Nat) : x / B + y / B ≤ (x + y) / B := by
  cases B with
  | zero => simp
  | succ B =>
    rw [Nat.le_div_iff_mul_le (Nat.succ_pos B), Nat.add_mul]
    exact Nat.add_le_add (Nat.div_mul_le_self x _) (Nat.div_mul_le_self y _)

theorem div_add_lt (x y B : Nat) (hB : 0 < B) : (x + y) / B < x / B + y / B + 2 := by
  rw [Nat.div_lt_iff_lt_mul hB]
  have hx := Nat.div_add_mod x B
  have hy := Nat.div_add_mod y B
  have rx := Nat.mod_lt x hB
  have ry := Nat.mod_lt y hB
  rw [Nat.add_mul, Nat.add_mul, Nat.mul_comm (x / B), Nat.mul_comm (y / B)]
  omega

/-- The rounded-down shares never add up to more than the whole … -/
theorem floors_le (a B : Nat) (ws : List Nat) : (floors a B ws).sum ≤ ws.sum * a / B := by
  induction ws with
  | nil => simp [floors]
  | cons w ws ih =>
    simp only [floors, List.map_cons, List.sum_cons] at *
    calc w * a / B + (ws.map (fun w => w * a / B)).sum ≤ w * a / B + ws.sum * a / B := Nat.add_le_add_left ih _
      _ ≤ (w * a + ws.sum * a) / B := add_div_le _ _ _
      _ = (w + ws.sum) * a / B := by rw [Nat.add_mul]

/-- … and fall short of it by fewer cents than there are lines. -/
theorem floors_short (a B : Nat) (hB : 0 < B) (ws : List Nat) (hn : 0 < ws.length) :
    ws.sum * a / B < (floors a B ws).sum + ws.length := by
  induction ws with
  | nil => simp at hn
  | cons w ws ih =>
    simp only [floors, List.map_cons, List.sum_cons, List.length_cons] at *
    cases ws with
    | nil => simp
    | cons v vs =>
      have ih' := ih (by simp)
      have h := div_add_lt (w * a) ((v :: vs).sum * a) B hB
      rw [← Nat.add_mul] at h
      omega

/-- With `B = Σ ws` the whole is the amount: the leftover cents `a − Σ floors`
are fewer than the lines, so each line takes at most one, and the shares add
up to `a` exactly. -/
theorem leftover_fits (a : Nat) (ws : List Nat) (hB : 0 < ws.sum) :
    (floors a ws.sum ws).sum ≤ a ∧ a < (floors a ws.sum ws).sum + ws.length := by
  have hall : ws.sum * a / ws.sum = a := Nat.mul_div_cancel_left a hB
  have hn : 0 < ws.length := by
    cases ws with
    | nil => simp at hB
    | cons _ _ => simp
  have h1 := floors_le a ws.sum ws
  have h2 := floors_short a ws.sum hB ws hn
  rw [hall] at h1 h2
  exact ⟨h1, h2⟩

end Karots

namespace Karots

/-- The leftover cents go one each to the largest remainders, ties to the
earlier line — the rule `sales.shareOut` and the till follow. -/
def insertBy (p : Nat × Nat) : List (Nat × Nat) → List (Nat × Nat)
  | [] => [p]
  | q :: qs => if p.1 > q.1 || (p.1 == q.1 && p.2 < q.2) then p :: q :: qs else q :: insertBy p qs

def shareOut (a : Nat) (ws : List Nat) : List Nat :=
  let B := ws.sum
  if B = 0 then ws.map (fun _ => 0) else
  let fs := floors a B ws
  let left := a - fs.sum
  -- (remainder, line) for the lines with a weight, largest remainder first
  let ranked := ((ws.zip (List.range ws.length)).filter (fun p => p.1 > 0)).foldr
    (fun p acc => insertBy (p.1 * a % B, p.2) acc) []
  let lucky := (ranked.take left).map Prod.snd
  (fs.zip (List.range ws.length)).map (fun (f, i) => if lucky.contains i then f + 1 else f)

end Karots
