# Idea — more AI uses (parked, user 2026-10-13)

M7.6 ships identify, tidy categories + possible duplicates, and explain-the-numbers. Parked for later, each
needs its own rethink:

- **Supplier bill photo → purchase lines.** Biggest win, needs image input (Gemini free tier takes images).
  The person checks every line before saving, like identify.
- **Write the order message.** From a PO or the reorder list, a short WhatsApp text to the supplier in
  English / Sinhala / Tamil.
- **Ask your shop.** A chat over the reports. Only safe if every figure comes from a core report (tool calls
  that return report rows), never from the model.
- **Sinhala / Tamil names** for labels and receipts.

**Reorder quantities stay core maths.** The AI explains the reorder list (Insights); it never sets quantities.

## MCP — skipped (user asked, 2026-10-13)

- The plugin doesn't need to be an MCP client: it hands the model the exact figures, which is safer than
  letting it fetch with tools.
- The suite as an MCP server (Claude Desktop etc. reading the shop) would open shop data to outside apps. Not
  for small shops now; reconsider after M8 alongside "Ask your shop".
