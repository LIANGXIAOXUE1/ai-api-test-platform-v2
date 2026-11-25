export async function downloadWord(url: string, filename: string) {
  const response = await fetch(url, { credentials: 'same-origin' })
  if (!response.ok) {
    const data = await response.json().catch(() => ({}))
    throw new Error(data.error || 'Word 报告导出失败，请重试')
  }
  if (!response.headers.get('content-type')?.includes('wordprocessingml')) throw new Error('服务未返回 Word 文档，请刷新后重试')
  const link = document.createElement('a'), objectURL = URL.createObjectURL(await response.blob())
  link.href = objectURL; link.download = filename; link.click()
  setTimeout(() => URL.revokeObjectURL(objectURL), 1000)
}
