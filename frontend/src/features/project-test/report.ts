// Kept independent from the page and assistant so both export the same report.
export async function downloadProjectTestReport(
  markdown: string | undefined,
  projectName: string,
  flash: (message: string) => void,
) {
  if (!markdown) return flash('还没有可导出的巡检报告')
  const fileName = `${projectName || 'project'}-接口联通性报告-${new Date().toISOString().slice(0, 10)}.md`
  const picker = (window as any).showSaveFilePicker
  if (typeof picker === 'function') {
    try {
      const handle = await picker({
        suggestedName: fileName,
        types: [{ description: 'Markdown 报告', accept: { 'text/markdown': ['.md'] } }],
      })
      const writer = await handle.createWritable()
      await writer.write(markdown)
      await writer.close()
      flash('报告已保存到你刚才选择的位置')
      return
    } catch (error) {
      if ((error as Error).name === 'AbortError') return flash('已取消保存报告')
      // 隐私模式或旧浏览器不支持另存为时，使用标准浏览器下载。
    }
  }
  const blob = new Blob([markdown], { type: 'text/markdown;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = fileName
  link.style.display = 'none'
  document.body.appendChild(link)
  link.click()
  link.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 1200)
  flash(`浏览器已开始下载“${fileName}”，请在下载列表中查看`)
}
