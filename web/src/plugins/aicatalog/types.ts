// Mirrors plugins/aicatalog/*.go JSON by hand (core's types.gen.ts is core-only).

export type Settings = {
  manualMode: boolean
  provider: string
  baseUrl: string
  model: string
  apiKey?: string // in only: blank = keep
  hasKey: boolean
  webSearch: boolean
  namingStyle: string
  customPrompt: string
}

/** Every AI action's answer: the result, or the prompt to copy (error says why when a paste or the API failed). */
export type AskReply<T> = { result?: T; prompt?: string; error?: string }

export type Candidate = { name: string; category: string; specs: string; explanation: string }
export type IdentifyResult = { confident: boolean; best: Candidate; options: Candidate[] | null }

type Why = { reason: string }
export type CleanupPlan = {
  renames: ({ id: number; to: string } & Why)[] | null
  reparents: ({ id: number; parent_id: number } & Why)[] | null
  merges: ({ from: number; into: number } & Why)[] | null
  moves: ({ product_id: number; category_id: number } & Why)[] | null
  uncategorized: ({ product_id: number; category: string } & Why)[] | null
  names: ({ product_id: number; to: string } & Why)[] | null
  duplicates?: null
}
export type DupProduct = { id: number; name: string; sales: number }
export type DupPair = { keep: DupProduct; drop: DupProduct; reason: string }
export type CleanupPreview = {
  plan: CleanupPlan
  categories: Record<string, string>
  products: Record<string, string>
  duplicates: DupPair[]
  chunk: number
  chunks: number
  next: number
}
export type Run = { id: number; summary: string }
export type EmptyCategory = { id: number; name: string }
