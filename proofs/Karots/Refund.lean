/-!
# Refunds of a line returned in parts (RET-4, RET-7)

Money is in whole cents, quantities in whole thousandths of a unit (the
database keeps 3 decimals), so everything is a natural number and nothing is
lost to floats.

A line was paid `P` cents for `Q` thousandths. When `n` more come back after
`R` already have, the refund is the rounded value of everything back so far,
less the rounded value of what was back before:

    refund P Q R n = roundShare P Q (R + n) - roundShare P Q R

so the roundings cancel: however the line comes back, the refunds never pass
what was paid, and once all of it is back they add up to it exactly.
-/

namespace Karots

/-- `P × m ÷ Q` rounded half up, in cents (Go: `Round(2)`, half away from zero). -/
def roundShare (P Q m : Nat) : Nat := (2 * P * m + Q) / (2 * Q)

/-- The refund for `n` more of a line when `R` were already back. -/
def refund (P Q R n : Nat) : Nat := roundShare P Q (R + n) - roundShare P Q R

/-- Refunds of a sequence of returns, starting with `R` already back. -/
def refunds (P Q R : Nat) : List Nat → Nat
  | [] => 0
  | n :: ns => refund P Q R n + refunds P Q (R + n) ns

/-- Division by the same number keeps the order. -/
theorem div_mono {a b k : Nat} (h : a ≤ b) : a / k ≤ b / k := by
  cases k with
  | zero => simp
  | succ k =>
    rw [Nat.le_div_iff_mul_le (Nat.succ_pos k)]
    exact Nat.le_trans (Nat.div_mul_le_self a _) h

theorem roundShare_mono (P Q : Nat) {a b : Nat} (h : a ≤ b) : roundShare P Q a ≤ roundShare P Q b := by
  unfold roundShare
  apply div_mono
  have : 2 * P * a ≤ 2 * P * b := Nat.mul_le_mul_left _ h
  omega

/-- Nothing back is worth nothing. -/
theorem roundShare_zero (P Q : Nat) (hQ : 0 < Q) : roundShare P Q 0 = 0 := by
  unfold roundShare
  simp
  exact Nat.div_eq_of_lt (by omega)

/-- All of it back is worth exactly what was paid. -/
theorem roundShare_all (P Q : Nat) (hQ : 0 < Q) : roundShare P Q Q = P := by
  unfold roundShare
  have h : 2 * P * Q + Q = (2 * P + 1) * Q := by rw [Nat.add_mul, Nat.one_mul]
  rw [h, Nat.mul_comm 2 Q, ← Nat.div_div_eq_div_mul, Nat.mul_div_cancel _ hQ]
  omega

/-- The refunds telescope: they add up to the value of what came back. -/
theorem refunds_telescope (P Q R : Nat) (ns : List Nat) :
    refunds P Q R ns = roundShare P Q (R + ns.sum) - roundShare P Q R := by
  induction ns generalizing R with
  | nil => simp [refunds]
  | cons n ns ih =>
    simp only [refunds, refund, ih, List.sum_cons]
    have h1 := roundShare_mono P Q (Nat.le_add_right R n)
    have h2 := roundShare_mono P Q (show R + n ≤ R + n + ns.sum from Nat.le_add_right _ _)
    rw [show R + (n + ns.sum) = R + n + ns.sum by omega]
    omega

/-- RET-4: however a line comes back, its refunds never pass what was paid … -/
theorem refunds_le_paid (P Q : Nat) (hQ : 0 < Q) (ns : List Nat) (h : ns.sum ≤ Q) :
    refunds P Q 0 ns ≤ P := by
  rw [refunds_telescope, Nat.zero_add, roundShare_zero P Q hQ, Nat.sub_zero]
  calc roundShare P Q ns.sum ≤ roundShare P Q Q := roundShare_mono P Q h
    _ = P := roundShare_all P Q hQ

/-- … and once all of it is back, they are exactly what was paid. -/
theorem refunds_all (P Q : Nat) (hQ : 0 < Q) (ns : List Nat) (h : ns.sum = Q) :
    refunds P Q 0 ns = P := by
  rw [refunds_telescope, Nat.zero_add, h, roundShare_zero P Q hQ, roundShare_all P Q hQ]
  rfl

end Karots
