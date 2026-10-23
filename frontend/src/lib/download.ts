import { strToU8, zipSync } from 'fflate'

function save(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

export const downloadText = (text: string, filename: string) =>
  save(new Blob([text], { type: 'text/plain;charset=utf-8' }), filename)

export function downloadZip(files: Record<string, string>, filename: string) {
  const zipped = zipSync(Object.fromEntries(Object.entries(files).map(([n, t]) => [n, strToU8(t)])))
  save(new Blob([zipped], { type: 'application/zip' }), filename)
}
