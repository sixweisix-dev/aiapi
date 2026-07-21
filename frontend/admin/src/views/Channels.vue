<template>
  <div>
    <!-- 🗂️ 渠道架构树形概览(分组 → 渠道 → 模型) -->
    <el-card shadow="hover" class="mb-4">
      <template #header>
        <div class="flex justify-between items-center">
          <span class="text-base font-medium">🗂️ 渠道架构</span>
          <div style="display:flex;gap:8px;">
            <el-button size="small" @click="openCreateGroup">+ 分组</el-button>
            <el-button size="small" @click="openCreateModel">+ 模型</el-button>
            <el-button size="small" type="primary" @click="openCreate">+ 渠道</el-button>
            <el-button size="small" type="primary" plain @click="refreshTree">🔄 刷新</el-button>
          </div>
        </div>
      </template>
      <el-tree
        v-if="treeData.length > 0"
        :data="treeData"
        node-key="id"
        :default-expand-all="true"
        :indent="20"
        :props="{ children: 'children', label: 'label' }"
      >
        <template #default="{ data }">
          <span style="display:flex;align-items:center;width:100%;">
            <span class="flex items-center" style="gap:8px;flex-wrap:wrap;flex:1;padding:4px 0;">
              <span>{{ data.type === 'group' ? '📦' : data.type === 'channel' ? '🔌' : '🤖' }}</span>
              <span class="font-medium" style="min-width:140px;">{{ data.label }}</span>
              <template v-if="data.type === 'group'">
                <el-tag size="small" type="success">{{ Number(data.meta.multiplier).toFixed(2) }}×</el-tag>
                <el-tag v-if="data.meta.is_default" size="small" type="warning">默认</el-tag>
                <span class="text-xs text-gray-400">{{ data.children?.length || 0 }} 渠道</span>
              </template>
              <template v-if="data.type === 'channel'">
                <el-tag size="small">{{ data.meta.provider }}</el-tag>
                <el-tag :type="data.meta.health === 'healthy' ? 'success' : data.meta.health === 'unhealthy' ? 'danger' : 'info'" size="small">{{ data.meta.health }}</el-tag>
                <el-tag size="small" type="info">权重 {{ data.meta.weight }}</el-tag>
                <span class="text-xs text-gray-400">{{ data.children?.length || 0 }} 模型</span>
              </template>
              <template v-if="data.type === 'model'">
                <el-tag size="small" type="info">${{ Number(data.meta.input).toFixed(3) }}/M in</el-tag>
                <el-tag size="small" type="info">${{ Number(data.meta.output).toFixed(3) }}/M out</el-tag>
                <el-tag v-if="!data.meta.enabled" size="small" type="danger">已禁用</el-tag>
              </template>
            </span>
            <span style="margin-left:auto;display:flex;gap:4px;">
              <template v-if="data.type === 'group'">
                <el-button link size="small" @click.stop="openEditGroup(data.raw)">编辑</el-button>
                <el-button link size="small" type="danger" @click.stop="handleDeleteGroup(data.raw)">删除</el-button>
              </template>
              <template v-if="data.type === 'channel'">
                <el-button link size="small" @click.stop="openEdit(data.raw)">编辑</el-button>
              </template>
              <template v-if="data.type === 'model'">
                <el-button link size="small" @click.stop="openEditModel(data.raw)">编辑</el-button>
                <el-button link size="small" type="danger" @click.stop="handleDeleteModel(data.raw)">删除</el-button>
              </template>
            </span>
          </span>
        </template>
      </el-tree>
      <el-empty v-else description="暂无数据" :image-size="60" />
    </el-card>

    <!-- 原渠道列表 -->
    <el-card shadow="hover" class="mb-4">
      <div class="flex justify-between items-center">
        <div class="flex items-center" style="gap:12px;">
          <span class="text-base font-medium">上游渠道池</span>
          <el-select v-model="groupFilter" placeholder="全部分组" clearable style="width:200px;" size="small">
            <el-option label="全部分组" :value="null" />
            <el-option v-for="g in channelGroups" :key="g.id" :label="`${g.name} (${Number(g.multiplier).toFixed(2)}×)`" :value="g.id" />
            <el-option label="未分组" :value="0" />
          </el-select>
        </div>
        <el-button type="primary" @click="openCreate">添加渠道</el-button>
      </div>
    </el-card>

    <el-card shadow="hover">
      <el-table :data="filteredChannels" v-loading="loading" stripe style="width: 100%">
        <el-table-column prop="name" label="名称" min-width="140">
          <template #default="{ row }">
            <div>{{ row.name }}</div>
            <el-tag v-if="row.is_dedicated" size="small" type="warning" effect="dark" class="mt-1">专属</el-tag>
            <el-tag v-if="row.dedicated_user_ids_auto" size="small" type="warning" plain class="mt-1 ml-1">🟡 自动 {{ countUsers(row.dedicated_user_ids_auto) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="provider" label="提供商" width="100">
          <template #default="{ row }"><el-tag size="small">{{ row.provider }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="weight" label="权重" width="70" align="center" />
        <el-table-column label="分组" width="120">
          <template #default="{ row }">
            <el-tag v-if="row.group_name" type="primary" size="small">{{ row.group_name }}</el-tag>
            <span v-else class="text-xs text-gray-400">未分组</span>
          </template>
        </el-table-column>

        <el-table-column label="额度使用" min-width="240">
          <template #default="{ row }">
            <div v-if="row.quota_type === 'daily' && row.daily_quota_usd > 0">
              <div class="flex justify-between text-xs mb-1">
                <span>每日 {{ formatUSD(realUsed(row, row.quota_used_today_usd)) }} / {{ formatUSD(row.daily_quota_usd) }}</span>
                <span :style="{color: quotaColor(row)}">{{ quotaPercent(row) }}%</span>
              </div>
              <el-progress :percentage="Math.min(100, quotaPercent(row))" :color="quotaColor(row)" :stroke-width="8" :show-text="false" />
            </div>
            <div v-else-if="row.quota_type === 'fixed' && row.total_quota_usd > 0">
              <div class="flex justify-between text-xs mb-1">
                <span>固定 {{ formatUSD(realUsed(row, row.used_total_usd)) }} / {{ formatUSD(row.total_quota_usd) }}</span>
                <span :style="{color: quotaColor(row)}">{{ quotaPercent(row) }}%</span>
              </div>
              <el-progress :percentage="Math.min(100, quotaPercent(row))" :color="quotaColor(row)" :stroke-width="8" :show-text="false" />
            </div>
            <span v-else class="text-gray-400 text-xs">不限额</span>
          </template>
        </el-table-column>

        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="quotaTagType(row.quota_status)" size="small">{{ quotaLabel(row.quota_status) }}</el-tag>
            <el-tag v-if="row.health_status === 'unhealthy'" type="danger" size="small" class="ml-1">不健康</el-tag>
          </template>
        </el-table-column>

        <el-table-column label="1h 错误" width="100" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.errors_1h > 0" :type="row.errors_1h >= 30 ? 'danger' : 'warning'" size="small">
              ⚠ {{ row.errors_1h }}
            </el-tag>
            <span v-else class="text-gray-400">-</span>
          </template>
        </el-table-column>

        <el-table-column label="缓存命中" width="100" align="center">
          <template #default="{ row }">
            <span v-if="row.cache_total_tokens > 0">{{ (row.cache_hit_rate * 100).toFixed(1) }}%</span>
            <span v-else class="text-gray-400">-</span>
          </template>
        </el-table-column>

        <el-table-column prop="daily_cost_cny" label="今日成本" width="100" align="right">
          <template #default="{ row }">${{ Number(row.daily_cost_cny || 0).toFixed(4) }}</template>
        </el-table-column>

        <el-table-column prop="monthly_cost_cny" label="本月成本" width="100" align="right">
          <template #default="{ row }">${{ Number(row.monthly_cost_cny || 0).toFixed(2) }}</template>
        </el-table-column>

        <el-table-column prop="error_streak" label="连失败" width="80" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.error_streak >= 3" type="danger" size="small">{{ row.error_streak }}</el-tag>
            <span v-else>{{ row.error_streak }}</span>
          </template>
        </el-table-column>

        <el-table-column prop="is_enabled" label="启用" width="70" align="center">
          <template #default="{ row }"><el-switch :model-value="row.is_enabled" disabled size="small" /></template>
        </el-table-column>

        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="openEdit(row)">编辑</el-button>
            <el-button size="small" type="warning" @click="handleResetQuota(row)">重置额度</el-button>
            <el-button size="small" type="success" :loading="testingId === row.id" @click="handleTest(row)">测试</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="isEditing ? '编辑渠道' : '添加渠道'" width="640px">
      <el-form :model="form" :rules="rules" ref="formRef" label-width="120px">
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="例如: 云翼主力账号" />
        </el-form-item>
        <el-form-item label="提供商" prop="provider">
          <el-select v-model="form.provider" style="width:100%">
            <el-option label="OpenAI" value="openai" />
            <el-option label="Anthropic" value="anthropic" />
            <el-option label="多模型聚合" value="multi_aggregator" />
            <el-option label="Google" value="google" />
            <el-option label="Qwen" value="qwen" />
            <el-option label="DeepSeek" value="deepseek" />
          </el-select>
        </el-form-item>
        <el-form-item label="API Key" prop="api_key">
          <el-input v-model="form.api_key" type="password" show-password :placeholder="isEditing ? '留空则不修改' : ''" />
        </el-form-item>
        <el-form-item label="自定义URL">
          <el-input v-model="form.base_url" placeholder="如 https://yunyi.rdzhvip.com/claude" />
        </el-form-item>
        <el-form-item label="权重">
          <el-input-number v-model="form.weight" :min="1" :max="100" />
          <span class="ml-2 text-xs text-gray-400">值越大被选中概率越高</span>
        </el-form-item>

        <el-form-item label="分组">
          <el-select v-model="form.group_id" placeholder="选择渠道分组" clearable style="width:240px">
            <el-option v-for="g in channelGroups" :key="g.id" :label="`${g.name} (${Number(g.multiplier).toFixed(2)}×)`" :value="g.id" />
          </el-select>
          <span class="ml-2 text-xs text-gray-400">渠道隶属哪个分组, 决定路由 + 倍率</span>
        </el-form-item>

        <el-form-item label="支持模型">
          <el-input v-model="form.supported_models" placeholder="留空=支持本组所有模型 / 限定示例: gpt-image-2,gpt-image-2-1k" style="width:480px" />
          <span class="ml-2 text-xs text-gray-400">逗号分隔模型名, 空白=承接分组所有模型</span>
        </el-form-item>

        <el-form-item label="故障转移优先级">
          <el-select v-model="fallbackList" multiple filterable placeholder="不设置=按 weight 兜底" style="width:480px">
            <el-option
              v-for="ch in fallbackCandidates"
              :key="ch.id"
              :label="`${ch.name} (${ch.provider})`"
              :value="ch.id"
            />
          </el-select>
          <div class="ml-2 text-xs text-gray-400" style="margin-top:4px;">
            该渠道 fail 后按选中顺序尝试。留空则回退到按 weight 排序的组内轮询。
          </div>
        </el-form-item>

        <el-divider>额度管理</el-divider>

        <el-form-item label="额度模式">
          <el-radio-group v-model="form.quota_type">
            <el-radio value="unlimited">不限额</el-radio>
            <el-radio value="daily">每日刷新型</el-radio>
            <el-radio value="fixed">固定总额型</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="每日额度 USD" v-if="form.quota_type === 'daily'">
          <el-input-number v-model="form.daily_quota_usd" :min="0" :step="10" />
          <span class="ml-2 text-xs text-gray-400">每天北京时间 0 点自动重置, 例如 100/200</span>
        </el-form-item>
        <el-form-item label="固定总额 USD" v-if="form.quota_type === 'fixed'">
          <el-input-number v-model="form.total_quota_usd" :min="0" :step="50" />
          <span class="ml-2 text-xs text-gray-400">用完即止, 不重置</span>
        </el-form-item>

        <el-form-item label="订阅开始" v-if="form.quota_type === 'daily'">
          <el-date-picker v-model="form.subscription_start" type="date" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>
        <el-form-item label="订阅结束" v-if="form.quota_type === 'daily'">
          <el-date-picker v-model="form.subscription_end" type="date" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>

        <el-divider>计费模式</el-divider>

        <el-form-item label="计费模式">
          <el-select v-model="form.billing_mode" style="width: 200px">
            <el-option label="按量计费 (pay-as-you-go)" value="pay_as_you_go" />
            <el-option label="包月套餐 (subscription)" value="subscription" />
          </el-select>
          <span class="ml-2 text-xs text-gray-400">subscription = 上游每月固定费用; pay_as_you_go = 按 token 计费</span>
        </el-form-item>

        <el-form-item label="月费 USD" v-if="form.billing_mode === 'subscription'">
          <el-input-number v-model="form.monthly_fee_cny" :min="0" :step="1" :precision="2" controls-position="right" />
          <span class="ml-2 text-xs text-gray-400">每月固定支付的费用 (利润看板会按 月费/30 摊销到每天)</span>
        </el-form-item>

        <el-divider>对账倍率（widget 用）</el-divider>

        <el-form-item label="对账倍率">
          <el-input-number v-model="form.reconcile_multiplier" :min="0.1" :max="10" :step="0.01" :precision="2" controls-position="right" />
          <span class="ml-2 text-xs text-gray-400">⚠ 仅用于「后台对账核算」,不影响用户实际付费单价。例:上游 USD × 此倍率 × 7 (汇率) = 应得 CNY,用于判断 channel 盈利。Aitechflux/Vertex 都应是 2.5。</span>
        </el-form-item>

        <el-divider>Cache 优化</el-divider>

        <el-form-item label="1h Cache Beta">
          <el-switch v-model="form.enable_cache_1h_beta" active-text="启用" inactive-text="关闭" />
          <span class="ml-2 text-xs text-gray-400">注入 anthropic-beta: extended-cache-ttl-2025-04-11 header, cache TTL 5min→60min。仅对真支持 cache 的上游 (Anthropic 直连) 有效, 反代池开了无用甚至亏损</span>
        </el-form-item>

        <el-form-item label="自动注入 Cache">
          <el-switch v-model="form.auto_inject_cache" active-text="启用" inactive-text="关闭" />
          <span class="ml-2 text-xs text-gray-400">网关层解析请求, 给长 system (≥4000 字符) 自动加 cache_control。客户端零改动即可享受 cache。配合 1h Beta 时使用 ttl:1h, 否则 5min</span>
        </el-form-item>

        <el-divider>专属配置</el-divider>

        <el-form-item label="专属渠道">
          <el-switch v-model="form.is_dedicated" />
          <span class="ml-2 text-xs text-gray-400">仅专属用户可路由到此渠道</span>
        </el-form-item>
        <el-form-item label="专属用户ID" v-if="form.is_dedicated">
          <el-input v-model="form.dedicated_user_ids" type="textarea" :rows="2" placeholder="逗号分隔, 如: uuid1, uuid2" />
        </el-form-item>
        <el-form-item label="自动隔离名单" v-if="form.is_dedicated && form.dedicated_user_ids_auto">
          <el-input v-model="form.dedicated_user_ids_auto" type="textarea" :rows="2" readonly />
          <div class="text-xs text-gray-400 mt-1">🟡 系统根据 30 分钟成本占比自动加入；每日 0 点重置</div>
        </el-form-item>

        <el-form-item label="启用" v-if="isEditing">
          <el-switch v-model="form.is_enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
      </template>
    </el-dialog>
  
    <!-- ========== 分组编辑 dialog ========== -->
    <el-dialog v-model="groupDialogVisible" :title="isEditingGroup ? '编辑分组' : '新建分组'" width="600px">
      <el-form :model="groupForm" :rules="groupRules" ref="groupFormRef" label-width="120px">
        <el-form-item label="名称" prop="name">
          <el-input v-model="groupForm.name" placeholder="例如: 经济版 / 官方直连" />
        </el-form-item>
        <el-form-item label="slug" prop="slug">
          <el-input v-model="groupForm.slug" :disabled="isEditingGroup" placeholder="economy / official" />
          <span class="text-xs text-gray-400 ml-2">内部标识, 创建后不可修改</span>
        </el-form-item>
        <el-form-item label="倍率" prop="multiplier">
          <el-input-number v-model="groupForm.multiplier" :min="0.01" :step="0.1" :precision="4" controls-position="right" />
          <span class="ml-2 text-xs text-gray-400">分组级倍率,影响该分组下所有模型。最终用户付费 = input_price × group × model 倍率。默认 1.0 = 不调整,例:VIP 分组 0.8x 打折。</span>
        </el-form-item>
        <el-form-item label="说明(中)"><el-input v-model="groupForm.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="英文名"><el-input v-model="groupForm.name_en" /></el-form-item>
        <el-form-item label="说明(英)"><el-input v-model="groupForm.description_en" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="groupForm.sort_order" :min="0" :step="1" /></el-form-item>
        <el-form-item label="默认分组"><el-switch v-model="groupForm.is_default" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSaveGroup">保存</el-button>
      </template>
    </el-dialog>

    <!-- ========== 模型编辑 dialog ========== -->
    <el-dialog v-model="modelDialogVisible" :title="isEditingModel ? '编辑模型' : '添加模型'" width="600px">
      <el-form :model="modelForm" :rules="modelRules" ref="modelFormRef" label-width="120px">
        <el-form-item label="模型标识" prop="name"><el-input v-model="modelForm.name" :disabled="isEditingModel" placeholder="如: gpt-4" /></el-form-item>
        <el-form-item label="显示名称" prop="display_name"><el-input v-model="modelForm.display_name" placeholder="如: GPT-4" /></el-form-item>
        <el-form-item label="提供商" prop="provider">
          <el-select v-model="modelForm.provider" style="width:100%" :disabled="isEditingModel">
            <el-option label="OpenAI" value="openai" />
            <el-option label="Anthropic" value="anthropic" />
            <el-option label="多模型聚合" value="multi_aggregator" />
            <el-option label="Google" value="google" />
            <el-option label="Qwen" value="qwen" />
            <el-option label="DeepSeek" value="deepseek" />
          </el-select>
        </el-form-item>
        <el-form-item label="上下文长度"><el-input-number v-model="modelForm.context_length" :min="1024" :step="4096" /></el-form-item>
        <el-row :gutter="10">
          <el-col :span="12"><el-form-item label="输入单价" prop="input_price"><el-input-number v-model="modelForm.input_price" :precision="6" :step="0.001" :min="0" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="输出单价" prop="output_price"><el-input-number v-model="modelForm.output_price" :precision="6" :step="0.001" :min="0" /></el-form-item></el-col>
        </el-row>
        <el-form-item label="分组">
          <el-select v-model="modelForm.group_id" placeholder="无分组" clearable style="width:280px">
            <el-option :value="null" label="— 无分组 —" />
            <el-option v-for="g in channelGroups" :key="g.id" :label="`${g.name} (${Number(g.multiplier).toFixed(2)}×)`" :value="g.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="上游别名"><el-input v-model="modelForm.upstream_name" placeholder="留空 = 同模型标识" /></el-form-item>
        <el-form-item label="倍率">
          <el-input-number v-model="modelForm.multiplier" :precision="2" :step="0.1" :min="0.5" />
          <span class="ml-2 text-xs text-gray-400">单模型倍率,只影响此模型。默认 1.0 = 不调整,例:促销期 0.8x 限时半价。</span>
        </el-form-item>
        <el-form-item label="描述"><el-input v-model="modelForm.description" type="textarea" :rows="2" /></el-form-item>
        <el-row :gutter="10">
          <el-col :span="12"><el-form-item label="公开"><el-switch v-model="modelForm.is_public" /></el-form-item></el-col>
          <el-col :span="12" v-if="isEditingModel"><el-form-item label="启用"><el-switch v-model="modelForm.is_enabled" /></el-form-item></el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="modelDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingModel" @click="handleSaveModel">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
const countUsers = (csv) => csv ? csv.split(',').filter(x => x.trim()).length : 0
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import dayjs from 'dayjs'
import api, { channelsAPI } from '../utils/api'

const channels = ref([])

const filteredChannels = computed(() => {
  const g = groupFilter.value
  if (g === null || g === undefined || g === '') return channels.value
  if (g === 0) return channels.value.filter(c => !c.group_id)
  return channels.value.filter(c => c.group_id === g)
})
const loading = ref(false)
const dialogVisible = ref(false)
const isEditing = ref(false)
const saving = ref(false)
const testingId = ref(null)
const formRef = ref(null)
const editingId = ref(null)

const form = reactive({
  name: '', provider: 'anthropic', api_key: '', base_url: '', weight: 1, is_enabled: true,
  quota_type: 'unlimited', daily_quota_usd: 0, total_quota_usd: 0,
  subscription_start: '', subscription_end: '',
  is_dedicated: false, dedicated_user_ids: '', reconcile_multiplier: 1.0, billing_mode: 'pay_as_you_go', monthly_fee_cny: 0, enable_cache_1h_beta: false, auto_inject_cache: false, group_id: null, fallback_channel_ids: ''
})

const rules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  provider: [{ required: true, message: '请选择提供商', trigger: 'change' }],
}

const channelGroups = ref([])
const groupFilter = ref(null)
async function loadGroups() {
  try { const r = await api.get('/admin/channel-groups'); channelGroups.value = r?.items || [] } catch (e) {}
}

// 故障转移优先级: 排除当前编辑的渠道 + 只显示 enabled 渠道
const fallbackCandidates = computed(() => {
  return channels.value.filter(c => c.is_enabled && c.id !== editingId.value)
})
// fallback_channel_ids 存储用逗号分隔字符串, UI 用数组
const fallbackList = computed({
  get() {
    return form.fallback_channel_ids ? form.fallback_channel_ids.split(',').filter(Boolean) : []
  },
  set(val) {
    form.fallback_channel_ids = Array.isArray(val) ? val.join(',') : ''
  }
})

const modelsList = ref([])
async function loadModels() {
  try { const r = await api.get('/admin/models'); modelsList.value = r?.items || [] } catch (e) {}
}
async function refreshTree() {
  await Promise.all([fetchData(), loadGroups(), loadModels()])
}

// Provider 兼容性: multi_aggregator 通配 (除 vertex_ai), 其他严格匹配
const isProviderCompatible = (channelProvider, modelProvider) => {
  if (!channelProvider || !modelProvider) return false
  if (channelProvider === 'multi_aggregator') return modelProvider !== 'vertex_ai'
  return channelProvider === modelProvider
}

// 渠道架构树形:分组 → 渠道 → 模型(每渠道下显示该分组下所有模型)
const treeData = computed(() => {
  if (!channelGroups.value.length) return []
  return channelGroups.value.map(g => {
    const gChannels = channels.value.filter(c => c.group_id === g.id)
    const gModels = modelsList.value.filter(m => m.group_id === g.id)
    const mkModel = (c, m) => ({
      id: `model-${c?.id || 'none'}-${m.id}`,
      type: 'model',
      label: m.display_name || m.name,
      meta: {
        input: (Number(m.input_price) || 0) * 1000 * (Number(m.multiplier) || 1) * (Number(g.multiplier) || 1),
        output: (Number(m.output_price) || 0) * 1000 * (Number(m.multiplier) || 1) * (Number(g.multiplier) || 1),
        enabled: m.is_enabled,
      },
      raw: m,
    })
    return {
      id: `group-${g.id}`,
      type: 'group',
      label: g.name,
      meta: { multiplier: g.multiplier, is_default: g.is_default },
      raw: g,
      children: gChannels.length > 0
        ? gChannels.map(c => ({
            id: `channel-${c.id}`,
            type: 'channel',
            label: c.name,
            meta: { provider: c.provider, health: c.health_status, weight: c.weight },
            raw: c,
            children: gModels.filter(m => {
              if (!isProviderCompatible(c.provider, m.provider)) return false
              // supported_models whitelist 过滤: 空表示支持本组所有
              if (c.supported_models && c.supported_models.trim() !== '') {
                const allowed = c.supported_models.split(',').map(s => s.trim()).filter(Boolean)
                return allowed.includes(m.name)
              }
              return true
            }).map(m => mkModel(c, m)),
          }))
        : gModels.map(m => mkModel(null, m)),
    }
  })
})

// ========== Channel Group CRUD ==========
const groupDialogVisible = ref(false)
const isEditingGroup = ref(false)
const editingGroupId = ref(null)
const groupFormRef = ref(null)
const groupForm = reactive({
  name: '', slug: '', multiplier: 1.0, description: '',
  name_en: '', description_en: '', sort_order: 0, is_default: false
})
const groupRules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  slug: [{ required: true, message: '请输入 slug', trigger: 'blur' }],
  multiplier: [{ required: true, message: '请输入倍率', trigger: 'blur' }],
}
function openCreateGroup() {
  isEditingGroup.value = false; editingGroupId.value = null
  Object.assign(groupForm, { name:'', slug:'', multiplier:1.0, description:'', name_en:'', description_en:'', sort_order:0, is_default:false })
  groupDialogVisible.value = true
}
function openEditGroup(row) {
  isEditingGroup.value = true; editingGroupId.value = row.id
  Object.assign(groupForm, {
    name: row.name, slug: row.slug, multiplier: Number(row.multiplier),
    description: row.description || '', name_en: row.name_en || '',
    description_en: row.description_en || '', sort_order: row.sort_order || 0,
    is_default: !!row.is_default
  })
  groupDialogVisible.value = true
}
async function handleSaveGroup() {
  await groupFormRef.value?.validate?.().catch(() => null)
  const payload = {
    name: groupForm.name, slug: groupForm.slug,
    multiplier: Number(groupForm.multiplier),
    description: groupForm.description || '',
    name_en: groupForm.name_en || '',
    description_en: groupForm.description_en || '',
    sort_order: Number(groupForm.sort_order) || 0,
    is_default: !!groupForm.is_default,
  }
  try {
    if (isEditingGroup.value) {
      await api.put(`/admin/channel-groups/${editingGroupId.value}`, payload); ElMessage.success('已更新')
    } else {
      await api.post('/admin/channel-groups', payload); ElMessage.success('已创建')
    }
    groupDialogVisible.value = false; refreshTree()
  } catch (e) { ElMessage.error('保存失败: ' + (e?.response?.data?.error || e.message)) }
}
async function handleDeleteGroup(row) {
  try { await ElMessageBox.confirm(`确定删除分组「${row.name}」?`, '确认', { type: 'warning' }) } catch { return }
  try {
    await api.delete(`/admin/channel-groups/${row.id}`); ElMessage.success('已删除'); refreshTree()
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '未知错误'
    if (msg.includes('channels') || msg.includes('models')) ElMessage.error('请先将该分组下的渠道/模型移到其他分组')
    else ElMessage.error('删除失败: ' + msg)
  }
}

// ========== Model CRUD ==========
const modelDialogVisible = ref(false)
const isEditingModel = ref(false)
const editingModelId = ref(null)
const modelFormRef = ref(null)
const savingModel = ref(false)
const modelForm = ref({
  name: '', display_name: '', provider: 'openai', context_length: 4096,
  input_price: 0, output_price: 0, multiplier: 1.0, description: '',
  is_public: true, is_enabled: true, group_id: null, upstream_name: ''
})
const modelRules = {
  name: [{ required: true, message: '请输入模型标识', trigger: 'blur' }],
  display_name: [{ required: true, message: '请输入显示名称', trigger: 'blur' }],
  provider: [{ required: true, message: '请选择提供商', trigger: 'change' }],
  input_price: [{ required: true, message: '请输入输入单价', trigger: 'blur' }],
  output_price: [{ required: true, message: '请输入输出单价', trigger: 'blur' }],
}
function openCreateModel() {
  isEditingModel.value = false; editingModelId.value = null
  modelForm.value = { name: '', display_name: '', provider: 'openai', context_length: 4096, input_price: 0, output_price: 0, multiplier: 1.0, description: '', is_public: true, is_enabled: true, group_id: null, upstream_name: '' }
  modelDialogVisible.value = true
}
function openEditModel(row) {
  isEditingModel.value = true; editingModelId.value = row.id
  modelForm.value = { ...row, upstream_name: row.upstream_name || '' }
  modelDialogVisible.value = true
}
async function handleSaveModel() {
  const valid = await modelFormRef.value?.validate().catch(() => false)
  if (!valid) return
  savingModel.value = true
  try {
    const data = {
      display_name: modelForm.value.display_name,
      input_price: modelForm.value.input_price,
      output_price: modelForm.value.output_price,
      multiplier: modelForm.value.multiplier,
      group_id: modelForm.value.group_id ? Number(modelForm.value.group_id) : 0,
      upstream_name: modelForm.value.upstream_name || null,
      is_public: modelForm.value.is_public,
      description: modelForm.value.description || null,
    }
    if (isEditingModel.value) {
      data.is_enabled = modelForm.value.is_enabled
      await api.put(`/admin/models/${editingModelId.value}`, data); ElMessage.success('更新成功')
    } else {
      await api.post('/admin/models', {
        name: modelForm.value.name, display_name: modelForm.value.display_name,
        provider: modelForm.value.provider, context_length: modelForm.value.context_length,
        input_price: modelForm.value.input_price, output_price: modelForm.value.output_price,
        multiplier: modelForm.value.multiplier,
        group_id: modelForm.value.group_id ? Number(modelForm.value.group_id) : null,
        is_public: modelForm.value.is_public, description: modelForm.value.description || null,
      })
      ElMessage.success('创建成功')
    }
    modelDialogVisible.value = false; refreshTree()
  } finally { savingModel.value = false }
}
async function handleDeleteModel(row) {
  try {
    await ElMessageBox.confirm(`确定要删除模型 "${row.display_name}" 吗?`, '提示')
    await api.delete(`/admin/models/${row.id}`); ElMessage.success('已删除'); refreshTree()
  } catch { /* cancelled */ }
}

onMounted(() => { fetchData(); loadGroups(); loadModels() })

async function fetchData() {
  loading.value = true
  try {
    const data = await channelsAPI.list()
    channels.value = data.items || []
  } catch { ElMessage.error('加载失败') } finally { loading.value = false }
}

// 反算上游真实消耗 USD (使用 reconcile_multiplier; 默认 1.0)
function realUsed(row, raw) {
  const m = Number(row.reconcile_multiplier) || 1.0
  return raw / m
}
function quotaPercent(row) {
  if (row.quota_type === 'daily' && row.daily_quota_usd > 0) {
    return Number((realUsed(row, row.quota_used_today_usd) / row.daily_quota_usd * 100).toFixed(1))
  }
  if (row.quota_type === 'fixed' && row.total_quota_usd > 0) {
    return Number((realUsed(row, row.used_total_usd) / row.total_quota_usd * 100).toFixed(1))
  }
  return 0
}
function quotaColor(row) {
  const p = quotaPercent(row)
  if (p >= 90) return '#dc2626'
  if (p >= 80) return '#f59e0b'
  if (p >= 50) return '#3b82f6'
  return '#10b981'
}
function quotaTagType(s) { return { normal:'success', warning:'warning', critical:'danger', exhausted:'info' }[s] || '' }
function quotaLabel(s) { return { normal:'正常', warning:'预警 80%', critical:'紧急 90%', exhausted:'已耗尽' }[s] || s }
function formatUSD(v) { return '$' + Number(v || 0).toFixed(2) }

function openCreate() {
  isEditing.value = false; editingId.value = null
  Object.assign(form, { name:'', provider:'anthropic', api_key:'', base_url:'', weight:1, is_enabled:true, quota_type:'unlimited', daily_quota_usd:0, total_quota_usd:0, subscription_start:'', subscription_end:'', is_dedicated:false, dedicated_user_ids:'', reconcile_multiplier:1.0, billing_mode:'pay_as_you_go', monthly_fee_cny:0, enable_cache_1h_beta:false, auto_inject_cache:false, group_id:null, account_balance_usd:0, supported_models:'' })
  dialogVisible.value = true
}

function openEdit(row) {
  isEditing.value = true; editingId.value = row.id
  Object.assign(form, {
    name: row.name, provider: row.provider, api_key: '',
    base_url: row.base_url || '', weight: row.weight, is_enabled: row.is_enabled,
    quota_type: row.quota_type || 'unlimited',
    daily_quota_usd: row.daily_quota_usd || 0,
    total_quota_usd: row.total_quota_usd || 0,
    subscription_start: row.subscription_start ? dayjs(row.subscription_start).format('YYYY-MM-DD') : '',
    subscription_end: row.subscription_end ? dayjs(row.subscription_end).format('YYYY-MM-DD') : '',
    is_dedicated: row.is_dedicated || false,
    dedicated_user_ids: row.dedicated_user_ids || '',
    dedicated_user_ids_auto: row.dedicated_user_ids_auto || '',
    reconcile_multiplier: Number(row.reconcile_multiplier) || 1.0,
    billing_mode: row.billing_mode || 'pay_as_you_go',
    monthly_fee_cny: Number(row.monthly_fee_cny) || 0,
    enable_cache_1h_beta: !!row.enable_cache_1h_beta,
    auto_inject_cache: !!row.auto_inject_cache,
    group_id: row.group_id || null,
    supported_models: row.supported_models || ''
  })
  dialogVisible.value = true
}

async function handleSave() {
  await formRef.value?.validate().catch(() => null)
  saving.value = true
  const payload = {
    name: form.name, provider: form.provider, weight: form.weight,
    quota_type: form.quota_type,
    daily_quota_usd: form.daily_quota_usd,
    total_quota_usd: form.total_quota_usd,
    is_dedicated: form.is_dedicated,
    dedicated_user_ids: form.dedicated_user_ids,
    reconcile_multiplier: Number(form.reconcile_multiplier) || 1.0,
    billing_mode: form.billing_mode || 'pay_as_you_go',
    monthly_fee_cny: Number(form.monthly_fee_cny) || 0,
    enable_cache_1h_beta: !!form.enable_cache_1h_beta,
    auto_inject_cache: !!form.auto_inject_cache,
    group_id: form.group_id ? Number(form.group_id) : 0,
    supported_models: form.supported_models || '',
  }
  if (form.api_key) payload.api_key = form.api_key
  if (form.base_url) payload.base_url = form.base_url
  if (form.subscription_start) payload.subscription_start = form.subscription_start
  if (form.subscription_end) payload.subscription_end = form.subscription_end
  if (isEditing.value) payload.is_enabled = form.is_enabled

  try {
    if (isEditing.value) {
      await channelsAPI.update(editingId.value, payload)
      ElMessage.success('更新成功')
    } else {
      await channelsAPI.create(payload)
      ElMessage.success('添加成功')
    }
    dialogVisible.value = false
    fetchData()
  } catch { ElMessage.error('保存失败') } finally { saving.value = false }
}

async function handleResetQuota(row) {
  await ElMessageBox.confirm(`重置 ${row.name} 的今日额度? 仅清零 quota_used_today.`, '提示', { type: 'warning' }).catch(() => null)
  try { await channelsAPI.update(row.id, { reset_quota: true }); ElMessage.success('已重置'); fetchData() } catch { ElMessage.error('重置失败') }
}

async function handleTest(row) {
  testingId.value = row.id
  try { await channelsAPI.test(row.id); ElMessage.success('连接正常') } catch { ElMessage.error('连接失败') } finally { testingId.value = null; fetchData() }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`删除 ${row.name}?`, '确认', { type: 'warning' }).catch(() => null)
  try { await channelsAPI.delete(row.id); ElMessage.success('已删除'); fetchData() } catch { ElMessage.error('删除失败') }
}
</script>

<style scoped>
.flex { display: flex; }
.justify-between { justify-content: space-between; }
.items-center { align-items: center; }
.mb-4 { margin-bottom: 16px; }
.mb-1 { margin-bottom: 4px; }
.mt-1 { margin-top: 4px; }
.ml-1 { margin-left: 4px; }
.ml-2 { margin-left: 8px; }
.text-base { font-size: 14px; }
.text-xs { font-size: 12px; }
.text-gray-400 { color: #9ca3af; }
.font-medium { font-weight: 500; }
</style>
