import bcrypt from 'bcryptjs'
import type { CopyPerson } from '@/api/types.gen'

/** Offline sign-in: the PIN against the hash in the copy (cashiers and managers only). */
export const checkPin = (person: CopyPerson, pin: string) => bcrypt.compare(pin, person.pinHash)
