import { createCn } from 'cn/config'

/** clsx + tailwind-merge, told that the till type scale (text-till-*, app.css) is a
 * font size, not a colour — or a text colour after it would drop it. */
export const cn = createCn({
  extend: { classGroups: { 'font-size': [{ text: ['till-micro', 'till-xs', 'till-sm', 'till-body', 'till-lead', 'till-title', 'till-xl', 'till-price', 'till-big', 'till-total', 'till-hero', 'till-icon'] }] } },
})
