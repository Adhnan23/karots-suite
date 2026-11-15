/** A frozen product's goods are either out for sale (not counted while frozen)
 * or in storage (counted). Write-offs and conversions ask which. */
export type Where = '' | 'sale' | 'storage'

export function SaleOrStorage({ label, value, onChange, sale, storage }: {
  label: string
  value: Where
  onChange: (v: Where) => void
  sale: [string, string] // title, what it does
  storage: [string, string]
}) {
  return (
    <div role="radiogroup" aria-label={label} className="flex flex-col gap-1.5">
      <span className="text-[13px] font-semibold text-muted-foreground">{label}</span>
      <div className="grid grid-cols-2 gap-1.5">
        {([['sale', ...sale], ['storage', ...storage]] as const).map(([v, title, sub]) => (
          <button key={v} type="button" role="radio" aria-checked={value === v} onClick={() => onChange(v)}
            className="flex flex-col items-start rounded-[var(--radius-sm)] border border-border bg-muted px-3 py-2 text-left aria-checked:border-brand aria-checked:bg-brand aria-checked:text-brand-foreground">
            <b className="text-[13px]">{title}</b>
            <small className="text-[11.5px] opacity-75">{sub}</small>
          </button>
        ))}
      </div>
    </div>
  )
}

/** The write-off question for a frozen product. */
export const WriteOffWhere = ({ value, onChange }: { value: Where; onChange: (v: Where) => void }) => (
  <SaleOrStorage label="Frozen: was it out for sale or in storage?" value={value} onChange={onChange}
    sale={['Out for sale', 'stock stays as counted']} storage={['In storage', 'comes out of the counted stock']} />
)

/** A conversion into a frozen product: where did the pieces go? */
export const ConvertWhere = ({ name, value, onChange }: { name: string; value: Where; onChange: (v: Where) => void }) => (
  <SaleOrStorage label={`${name} is frozen: where did they go?`} value={value} onChange={onChange}
    sale={['Out for sale', 'not counted while frozen']} storage={['Into storage', 'added to the counted stock']} />
)
