# Repairs

Repair jobs from drop-off to pickup.

![Repairs](../../images/plugins/repairs.jpg)

- A **job**: device, fault, parts and labour, promised date and warranty. Print a ticket label and a slip for the customer.
- **Deposits** are the customer's advance. **Pickup** goes through the normal till and Pay, so a repair's money is in the same reports as everything else.
- The outside repairer is paid as an **expense** tagged to the repair (cash or cheque), so **Profit** per job is right. If that cheque bounces, the job shows the repairer as not paid again.
- Jobs move through **Received → In progress → Ready → Collected** (or Cancelled). Overdue jobs stand out. A collected or
  cancelled job is closed: its details, parts and labour can't change any more. Returning a pickup's items through
  Returns refunds them but leaves the job collected, with its warranty.
- **Warranty**: a job that comes back under its warranty is tracked, and its re-repair is free.
- The **report** shows each collected job's own bill lines, so two jobs picked up on one bill (or a part sold loose
  beside them) never count each other's.

The admin page is under **Apps → Repairs** (`/admin/x/repairs`). A shop has this plugin only if it was built with it ([Install & Shop Builder](../install.md)).
