# Idea: recharge through modems (user, 2026-10-18)

Today the cashier does the real reload / bill / transfer on a phone, then records it in the suite. The user's plan
for later: plug modems (local vendors sell multi-SIM GSM modems) holding the shop's carrier SIMs into the server,
and drive them through **USSD** or **SIM Toolkit**, so the suite itself

- sends the reload / package / transfer when the cart line is paid (no double entry, no wrong amounts), and
- reads the float balance (USSD balance query) — an automatic count, so drift and commission show up at once.

Fit with M7.9: a **device** is already the unit of float, so a modem is a device with a connection. A driver
would sit behind the device (send, balance); the till keeps the same cart lines and Pay. Open questions for then:
carrier USSD menus change without notice, STK needs AT commands per modem model, failures mid-transaction (the
sale must still never be stopped — record and flag), and whether carriers allow agent SIMs in modems.
Not planned yet — the user will bring the plan.
