import { expect, test } from '@playwright/test'

const realAdminURL = 'http://127.0.0.1:18080/admin/'

test('真实 Gin、SQLite 与 Vue 完成登录、读取和运营写入闭环', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', '真实服务闭环只需在一个桌面浏览器执行一次')

  const overviewResponse = page.waitForResponse((response) =>
    response.url().includes('/api/admin/overview') && response.request().method() === 'GET',
  )
  await page.goto(realAdminURL)
  await expect(page.getByRole('heading', { name: '宠物养成' })).toBeVisible()

  await page.getByLabel('管理员账号').fill('admin')
  await page.getByLabel('管理员密码', { exact: true }).fill('AdminPass!2026')
  if (await page.getByLabel('确认管理员密码').isVisible()) {
    await page.getByLabel('确认管理员密码').fill('AdminPass!2026')
  }
  await page.getByRole('button', { name: /设置密码并进入后台|登录/ }).click()

  if (await page.getByRole('heading', { name: '先选择机器人接入方式' }).isVisible()) {
    await page.getByRole('button', { name: '继续' }).click()
    await page.getByRole('button', { name: '进入运营后台' }).click()
  }

  await expect(page.getByRole('heading', { name: '宠物远征运营总览' })).toBeVisible()
  const skipTour = page.getByRole('button', { name: '跳过导览' }).first()
  if (await skipTour.isVisible()) await skipTour.click()
  const overviewPayload = await (await overviewResponse).json()
  expect(overviewPayload).toMatchObject({ code: 0, msg: 'success', data: { players: 1, pets: 1 } })
  expect(overviewPayload.request_id).toEqual(expect.any(String))

  await page.goto(`${realAdminURL}players`)
  await expect(page.getByRole('heading', { name: '玩家管理', exact: true })).toBeVisible()
  await expect(page.getByText('诺诺', { exact: true }).first()).toBeVisible()
  await page.getByRole('button', { name: '查看详情' }).click()
  const playerDrawer = page.getByRole('dialog', { name: '玩家全局存档' })
  await expect(playerDrawer.getByText('诺诺族', { exact: true })).toBeVisible()

  await page.getByRole('button', { name: '补发固定物品' }).click()
  await page.getByPlaceholder('必填，将写入审计日志').fill('真实集成测试补发')
  await page.getByRole('button', { name: '确认执行' }).click()
  await expect(page.getByText('操作已完成并写入审计日志')).toBeVisible()

  await page.getByRole('button', { name: '背包', exact: true }).click()
  await expect(page.getByText('调查记录', { exact: true })).toBeVisible()
  await expect(page.getByText('× 12', { exact: true })).toBeVisible()
})
