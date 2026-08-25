import AxeBuilder from '@axe-core/playwright'
import { expect, test, type Page } from '@playwright/test'

async function mockAdminApi(page: Page) {
  await page.route('**/api/admin/**', async (route) => {
    const request=route.request(),path=new URL(request.url()).pathname
    const ok=(data:unknown)=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({code:0,msg:'success',data})})
    if(path==='/api/admin/auth/session') return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({authenticated:true,username:'admin'})})
    if(path==='/api/admin/overview') return ok({range:'7d',players:12,pets:10,active_expeditions:4,completed_expeditions:18,active_communities:3,boss_participants:8,overdue_expeditions:1,command_success_rate:.98,command_total:800,generated_at:new Date().toISOString()})
    if(path==='/api/admin/players') return ok({items:[{account_id:'00000000-0000-0000-0000-000000123456',pet_name:'米塔',pet_type:'猫',role:'探索者',growth:120,bond_level:3,identity_count:2,community_count:1,expedition_id:'exp-1',expedition_status:'running',last_active_at:new Date().toISOString()}],total:1,page:1,limit:50})
    if(path.includes('/api/admin/players/')) return ok({account:{ID:'00000000-0000-0000-0000-000000123456'},pet:{Name:'米塔',Role:'探索者'},inventory:[{ID:1,ItemName:'调查记录',Quantity:20}],codex:[],identities:[{id:1,platform:'qq_group',scene_type:'group',subject_id:'***9876'}],expeditions:[],communities:[],notifications:{Enabled:true}})
    if(path==='/api/admin/expeditions') return ok({items:[{ID:'exp-1',AccountID:'account-1',Name:'遗迹调查',Tier:2,Stance:'守护',Status:'running',EndsAt:new Date().toISOString(),RewardItem:'古代零件',RewardQuantity:3,RewardRecords:12}],total:1,page:1,limit:100,summary:{running:1,today_completed:3,overdue:0,cancelled:1}})
    if(path==='/api/admin/gameplay/distributions') return ok({roles:[{name:'探索者',count:8}],stances:[{name:'守护',count:5}],traits:[{name:'温柔',count:3}],codex:[{name:'地区 · 森林',count:9}]})
    if(path==='/api/admin/gameplay/growth') return ok({summary:{player_count:0,role_coverage_rate:0,personality_formation_rate:0,personality_unformed:0,configured_rule_count:14,configuration_complete:true},roles:[{name:'探索者',count:0,percentage:0,description:'擅长寻路、观察与环境采样',enabled:true,skills:'寻路、观察、采样'}],stances:[{name:'守护',count:0,percentage:0,description:'降低事件风险',enabled:true}],personalities:[{name:'温柔',count:0,percentage:0,description:'长期照料行为更突出',enabled:true}],skills:[{name:'探索者技能组',count:0,percentage:0,description:'寻路、观察、采样',enabled:true}],rules:{roles:[{name:'探索者',description:'擅长寻路、观察与环境采样',skill_1:'寻路',skill_2:'观察',skill_3:'采样',enabled:true,sort_order:10}],stances:[{name:'守护',description:'降低事件风险',enabled:true,sort_order:10}],personalities:[{name:'温柔',dimension:'care',min_threshold:3,description:'长期照料行为更突出',enabled:true,sort_order:10}],codex:[{id:1,category:'生物',entry_key:'林间足迹',region:'森林',description:'森林调查记录',enabled:true,sort_order:10}]},warnings:[]})
    if(path==='/api/admin/gameplay/codex') return ok({summary:{catalog_count:1,discovered_entries:0,discovery_rate:0},items:[{id:1,category:'生物',entry_key:'林间足迹',region:'森林',description:'森林调查记录',enabled:true,sort_order:10,discovered_players:0,completed_players:0,average_progress:0}],warnings:[]})
    if(path==='/api/admin/communities') return ok({items:[{ID:'qq_group:app:group-1',Platform:'qq_group',SceneType:'group',Level:2,Materials:300,NotificationsEnabled:true,member_count:8,squad_count:2,open_help_count:1}],total:1,page:1,limit:100})
    if(path.startsWith('/api/admin/communities/')) return ok({community:{ID:'qq_group:app:group-1',Level:2,Materials:300,NotificationsEnabled:true},members:[],facilities:[],squads:[],bosses:[],help_requests:[],votes:[]})
    if(path==='/api/admin/platforms/status') return ok({onebot:{connected:true,connection_count:1},qq_official:{configured:true,connected:true,masked_app_id:'***6789',app_secret_configured:true,session_state:'running',connected_shards:1,recommended_shards:1,queue_depth:0,capabilities:{group:true,guild:true,markdown:false,keyboard:false,interaction:false}}})
    if(path==='/api/admin/groups') return ok([])
    if(path==='/api/admin/content/messages') return ok([{key:'pet.status',description:'宠物状态',template:'🐾 {pet_name}的近况',variables:{pet_name:'诺诺'},sample:'🐾 诺诺的近况'}])
    if(path.startsWith('/api/admin/content/messages/')&&path.endsWith('/preview')) return ok({key:'pet.status',platform:'onebot',sender:'QQ 宠物机器人',text:'🐾 诺诺的近况'})
    if(path.startsWith('/api/admin/config/')&&path.endsWith('/meta')) return ok({schema:path.split('/').at(-2),consumers:['统一领域服务'],effective_revision:2,db_revision:2,pending_reload:false})
    if(path==='/api/admin/config/live_events') return ok([{id:1,key:'forest-week',name:'森林调查周',region:'森林',story_choices:'["记录线索","继续调查","呼叫支援"]',starts_at:'2026-08-12T00:00:00Z',ends_at:'2026-08-19T00:00:00Z',active:true}])
    if(path==='/api/admin/config/reward_tracks') return ok([{id:1,event_key:'forest-week',milestone:100,item_name:'调查记录',quantity:10,description:'基础调查奖励'}])
    if(path==='/api/admin/config/pet_species') return ok([{Name:'诺诺',Image:'',FavoriteFood:'小鱼干',FavoriteGift:'铃铛',Health:100,Wisdom:20,Strength:18,Defense:16,Hunger:100,Description:'活泼的探索伙伴'}])
    if(path==='/api/admin/config/items') return ok([{Name:'调查记录',Status:'active',Type:'材料',Effect:'',Image:'',Description:'记录探索线索',SellPrice:0}])
    if(path==='/api/admin/config/shop_items') return ok([{ID:1,ShopType:'shop_normal',Name:'调查记录',Stock:20,RestockTarget:50,Price:10,Description:'基础调查用品'}])
    if(path==='/api/admin/config/images') return ok([{Name:'默认宠物图',Path:'pet.png'}])
    if(path==='/api/admin/config/commands') return ok([{FuncName:'status',Command:'状态',DisplayName:'查看宠物状态',Category:'基础',Description:'查看宠物近况与当前远征',Enabled:true,SortOrder:1}])
    if(path==='/api/admin/config/menus') return ok([{Name:'主菜单',Reply:'【宠物远征生态】\n状态｜今日｜远征'}])
    if(path==='/api/admin/settings/game') return ok([{key:'Core.InitialCoin',label:'初始货币',group:'基础设置',type:'number',unit:'枚',description:'新玩家建立存档时获得的基础货币。',value:100}])
    if(path==='/api/admin/platforms/config') return ok({listen_address:'127.0.0.1',port:8080,onebot:{token_configured:true},qq_official:{app_id:'12345678',app_secret_configured:true,api_base:'https://api.sgroup.qq.com',token_url:'https://bots.qq.com/app/getAppAccessToken',shard_count:1,markdown_enabled:false,keyboard_enabled:false,interaction_enabled:false,audit_enabled:false,group_events_enabled:true,guild_events_enabled:true}})
    if(path==='/api/admin/config/system') return ok([{Key:'Text.Welcome',Value:'欢迎来到宠物远征生态'}])
    if(path==='/api/admin/config/work_settings') return ok([{Name:'旧打工',Time:60,HungerCost:10,RewardCoin:20,RewardItems:''}])
    if(path==='/api/admin/config/checkin_rewards') return ok([{ID:1,Type:'checkin_weekly',Day:'1',Currency:10,Affection:1,Items:''}])
    if(path==='/api/admin/config/status') return ok({db_revision:2,loaded_revision:2,pending_reload:false,saved_at:null,loaded_at:null})
    if(path==='/api/admin/config/profiles') return ok({items:[{id:'official',name:'官方默认 v0.0.1',description:'内置安全默认配置',source:'official',schema_version:1,app_version:'0.0.1',builtin:true,active:true,dirty:false,summary:{schemas:18,rows:378},created_at:new Date().toISOString(),updated_at:new Date().toISOString()}],active_profile_id:'official',dirty:false})
    if(path==='/api/admin/onboarding/status') return ok({setup_completed:true,tour_version_completed:1,current_tour_version:1})
    if(path==='/api/admin/audit-logs') return ok({items:[],total:0,page:1,limit:50})
    return ok(null)
  })
}

test.beforeEach(async({page})=>{await mockAdminApi(page)})

test('八大领域导航与核心数据可用',async({page})=>{
  await page.goto('')
  await expect(page.getByRole('heading',{name:'宠物远征运营总览'})).toBeVisible()
  for(const [path,heading] of [['players','玩家管理'],['gameplay','玩法运营'],['communities','社群运营'],['profiles','配置方案'],['platforms','机器人平台状态'],['system','系统设置']] as const){await page.goto(path);await expect(page.getByRole('heading',{name:heading,exact:true}).first()).toBeVisible()}
  await page.goto('content')
  await expect(page.getByRole('button',{name:'活动运营'})).toBeVisible()
  await expect(page.getByRole('button',{name:'宠物与物品'})).toBeVisible()
})

test('成长零数据与内容配置均为完整产品页面',async({page},testInfo)=>{
  await page.goto('gameplay?tab=growth')
  await expect(page.getByText('暂无玩家行为数据，成长规则已经可以配置')).toBeVisible()
  await expect(page.getByText('实际玩法配置')).toBeVisible()
  await page.getByRole('button',{name:/探索者/}).last().click()
  await expect(page.getByText('固定技能组')).toBeVisible()
  await page.getByRole('button',{name:'关闭'}).click()

  await page.goto('content?tab=events')
  await page.locator('.event-card').filter({hasText:'森林调查周'}).getByRole('button',{name:'编辑活动'}).click()
  await expect(page.getByRole('dialog',{name:/编辑活动/})).toBeVisible()
  await page.getByRole('button',{name:'关闭'}).click()
  await page.getByRole('button',{name:'新建活动'}).first().click()
  await expect(page.getByText('使用说明')).toBeVisible()
  await expect(page.getByRole('heading',{name:'故事选项'})).toBeVisible()
  await expect(page.getByRole('heading',{name:'里程碑奖励'})).toBeVisible()
  await expect(page.getByText('累计奖励预览')).toBeVisible()
  await page.getByRole('button',{name:'关闭'}).click()

  await page.getByRole('button',{name:'宠物与物品'}).click()
  await page.locator('.pet-card').filter({hasText:'诺诺'}).click()
  await expect(page.getByRole('dialog',{name:'编辑宠物种类'})).toBeVisible()
  await page.getByRole('button',{name:'关闭'}).click()
  await page.getByRole('button',{name:'物品',exact:true}).click()
  if(testInfo.project.name==='mobile') await page.locator('.mobile-list .summary-card').filter({hasText:'调查记录'}).click()
  else await page.locator('.desktop-table tr').filter({hasText:'调查记录'}).getByRole('button',{name:'编辑'}).click()
  await expect(page.getByRole('dialog',{name:'编辑物品'})).toBeVisible()
  await page.getByRole('button',{name:'关闭'}).click()

  await page.getByRole('button',{name:'商店'}).click()
  await expect(page.locator('[aria-label="调查记录价格"]:visible')).toBeVisible()
  await page.getByRole('button',{name:'图片资产'}).click()
  await expect(page.getByText('默认宠物图')).toBeVisible()

  await page.getByRole('button',{name:'文本与命令'}).click()
  await page.locator('.command-row').filter({hasText:'查看宠物状态'}).click()
  await expect(page.getByRole('dialog',{name:'编辑命令'})).toBeVisible()
})

test('玩家、玩法和平台关键操作入口存在',async({page})=>{
  await page.goto('players');const detailButton=page.getByRole('button',{name:'详情'});if(await detailButton.isVisible())await detailButton.click();else await page.locator('.summary-card').first().click();await expect(page.getByRole('button',{name:'补发固定物品'})).toBeVisible()
  await page.goto('gameplay');await expect(page.getByRole('button',{name:'取消'})).toBeVisible();await expect(page.getByRole('button',{name:'结算'})).toBeVisible()
  await page.goto('platforms');await expect(page.getByRole('button',{name:'重新连接网关'})).toBeVisible();await expect(page.getByText('***6789')).toBeVisible();await expect(page.getByText(/your_app_secret/)).toHaveCount(0)
})

test('当前视口无业务横向溢出且无严重可访问性问题',async({page})=>{
  for(const path of ['','players','gameplay','communities','content','platforms']){
    await page.goto(path)
    expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(await page.evaluate(()=>window.innerWidth))
    const coveredByMobileNav=await page.evaluate(()=>{
      if(window.innerWidth>700)return [] as string[]
      const content=document.querySelector<HTMLElement>('.content'),nav=document.querySelector<HTMLElement>('.mobile-nav')
      if(!content||!nav)return ['缺少内容区或移动导航']
      window.scrollTo(0,document.documentElement.scrollHeight)
      content.scrollTop=content.scrollHeight
      const navRect=nav.getBoundingClientRect()
      return [...content.querySelectorAll<HTMLElement>('button,a,input,select,textarea')]
        .filter(element=>element.offsetParent!==null&&!element.closest('[role="dialog"]'))
        .filter(element=>{const rect=element.getBoundingClientRect();return rect.width>0&&rect.height>0&&rect.bottom>navRect.top&&rect.top<navRect.bottom})
        .map(element=>`${element.tagName.toLowerCase()}.${element.className}:${element.textContent?.trim().slice(0,30)}`)
    })
    expect(coveredByMobileNav, `${path||'dashboard'} 有操作被底部导航遮挡`).toEqual([])
    const result=await new AxeBuilder({page}).analyze()
    expect(result.violations.filter(item=>item.impact==='critical')).toEqual([])
  }
})

test('三套主题关键页面截图验收',async({page},testInfo)=>{
  for(const theme of ['mita-day','mita-night','mita-other']){
    for(const path of ['','players','content','platforms']){
      await page.goto(path)
      await page.evaluate(value=>{localStorage.setItem('adminTheme',value);document.documentElement.dataset.theme=value},theme)
      await page.screenshot({path:testInfo.outputPath(`${theme}-${path||'dashboard'}.png`),fullPage:true})
    }
  }
})
