export type AvailableModel = { id: number; enabled: boolean; verified?: boolean }
export function verifiedModels<T extends AvailableModel>(models: T[]): T[] {
  return models.filter(model => model.enabled === true && model.verified === true)
}
export function eligibleSelection(ids: number[], models: AvailableModel[]) {
  const available = new Set(verifiedModels(models).map(model => model.id))
  return ids.filter(id => available.has(id))
}
