// Mirrors plugins/alternatives/api.go by hand (core's types.gen.ts is core-only).

export type Member = { productId: number; name: string; stock: string; active: boolean; unit: string }
/** name '' = the one tier of a group that has no named tiers. */
export type Tier = { id: number; name: string; reorderLevel: number; active: boolean; total: string; low: boolean; members: Member[] }
export type Group = { id: number; name: string; active: boolean; products: number; tiers: Tier[] }
export type Alt = { productId: number; name: string; tier: string; stock: string; sellingPrice: string; wholesalePrice: string; unit: string; sameUnit: boolean }
export type Membership = { groupId: number; tierId: number; group: string; tier: string }
