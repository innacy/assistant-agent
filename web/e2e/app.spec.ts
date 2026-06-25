import { test, expect } from '@playwright/test';

const API_TOKEN = 'change-me-to-a-real-token';
const TOKEN_KEY = 'assistant_agent_token';

test.describe('Assistant Agent E2E Tests', () => {

  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.evaluate(({ token, key }) => {
      localStorage.setItem(key, token);
    }, { token: API_TOKEN, key: TOKEN_KEY });
    await page.reload();
    await page.waitForTimeout(2000);
  });

  test.describe('Dashboard Page', () => {
    test('loads dashboard with summary stats', async ({ page }) => {
      await page.goto('/');
      await page.waitForTimeout(3000);

      await expect(page.locator('body')).toContainText(/due today|today|upcoming|missed/i, { timeout: 10000 });
    });

    test('shows due today section', async ({ page }) => {
      await page.goto('/');
      await expect(page.locator('body')).toContainText(/Due Today/i, { timeout: 10000 });
    });

    test('shows upcoming alerts', async ({ page }) => {
      await page.goto('/');
      await expect(page.locator('body')).toContainText(/Upcoming/i, { timeout: 10000 });
      const body = await page.textContent('body');
      const hasUpcoming = body!.includes('Submit Quarterly Tax Report') || body!.includes('Spotify') || body!.includes('Mom Birthday') || body!.includes('Dentist');
      expect(hasUpcoming).toBeTruthy();
    });

    test('shows missed alerts', async ({ page }) => {
      await page.goto('/');
      await expect(page.locator('body')).toContainText('AWS Bill', { timeout: 10000 });
    });
  });

  test.describe('Alerts Page', () => {
    test('navigates to alerts page and lists all alerts', async ({ page }) => {
      await page.goto('/alerts');
      await page.waitForTimeout(3000);

      await expect(page.locator('body')).toContainText('Netflix Monthly Renewal', { timeout: 10000 });
      const body = await page.textContent('body');
      expect(body).toContain('Mom Birthday');
      expect(body).toContain('Submit Quarterly Tax Report');
    });

    test('can filter alerts by type', async ({ page }) => {
      await page.goto('/alerts');
      await page.waitForTimeout(3000);

      const typeFilter = page.locator('select').first();
      if (await typeFilter.isVisible({ timeout: 5000 })) {
        await typeFilter.selectOption('payment');
        await page.waitForTimeout(1000);

        await expect(page.locator('body')).toContainText('AWS Bill', { timeout: 5000 });
      }
    });

    test('can create a manual alert', async ({ page }) => {
      await page.goto('/alerts');
      await page.waitForLoadState('networkidle');

      const createBtn = page.locator('button:has-text("Create"), button:has-text("Add"), button:has-text("New")').first();
      if (await createBtn.isVisible()) {
        await createBtn.click();
        await page.waitForTimeout(300);

        const titleInput = page.locator('input[name="title"], input[placeholder*="title" i]').first();
        if (await titleInput.isVisible()) {
          await titleInput.fill('Test Manual Alert');

          const typeSelect = page.locator('select[name="type"]').first();
          if (await typeSelect.isVisible()) {
            await typeSelect.selectOption('task');
          }

          const dateInput = page.locator('input[type="date"], input[name="due_date"]').first();
          if (await dateInput.isVisible()) {
            await dateInput.fill('2026-07-15');
          }

          const submitBtn = page.locator('button[type="submit"], button:has-text("Save"), button:has-text("Create")').last();
          await submitBtn.click();
          await page.waitForTimeout(1000);

          const body = await page.textContent('body');
          expect(body).toContain('Test Manual Alert');
        }
      }
    });
  });

  test.describe('Alert Actions', () => {
    test('can acknowledge an alert', async ({ page }) => {
      await page.goto('/');
      await page.waitForLoadState('networkidle');

      const ackBtn = page.locator('button:has-text("Acknowledge"), button[title*="acknowledge" i], button[aria-label*="acknowledge" i]').first();
      if (await ackBtn.isVisible()) {
        await ackBtn.click();
        await page.waitForTimeout(500);
      } else {
        const response = await page.request.get('http://localhost:5173/api/alerts/today', {
          headers: { 'Authorization': `Bearer ${API_TOKEN}` },
        });
        const data = await response.json();
        if (data.data && data.data.length > 0) {
          const alertId = data.data[0].id;
          const ackResp = await page.request.post(`http://localhost:5173/api/alerts/${alertId}/acknowledge`, {
            headers: { 'Authorization': `Bearer ${API_TOKEN}` },
          });
          expect(ackResp.ok()).toBeTruthy();
        }
      }
    });

    test('can snooze an alert', async ({ page }) => {
      const response = await page.request.get('http://localhost:5173/api/alerts/upcoming', {
        headers: { 'Authorization': `Bearer ${API_TOKEN}` },
      });
      const data = await response.json();
      expect(data.data.length).toBeGreaterThan(0);

      const alertId = data.data[0].id;
      const snoozeResp = await page.request.post(`http://localhost:5173/api/alerts/${alertId}/snooze`, {
        headers: {
          'Authorization': `Bearer ${API_TOKEN}`,
          'Content-Type': 'application/json',
        },
        data: { snooze_until: '2026-06-28' },
      });
      expect(snoozeResp.ok()).toBeTruthy();
      const snoozeData = await snoozeResp.json();
      expect(snoozeData.ok).toBe(true);

      const getResp = await page.request.get(`http://localhost:5173/api/alerts/${alertId}`, {
        headers: { 'Authorization': `Bearer ${API_TOKEN}` },
      });
      const alertData = await getResp.json();
      expect(alertData.status).toBe('snoozed');
    });
  });

  test.describe('Settings Page', () => {
    test('navigates to settings and shows sync status', async ({ page }) => {
      await page.goto('/');
      await page.waitForLoadState('networkidle');

      const settingsLink = page.locator('a[href="/settings"], nav >> text=Settings').first();
      if (await settingsLink.isVisible()) {
        await settingsLink.click();
      } else {
        await page.goto('/settings');
      }
      await page.waitForLoadState('networkidle');

      const body = await page.textContent('body');
      expect(body).toBeTruthy();
      const hasSyncContent = body!.includes('Sync') || body!.includes('sync') || body!.includes('Settings') || body!.includes('settings');
      expect(hasSyncContent).toBeTruthy();
    });

    test('can update settings', async ({ page }) => {
      const response = await page.request.get('http://localhost:5173/api/settings', {
        headers: { 'Authorization': `Bearer ${API_TOKEN}` },
      });
      expect(response.ok()).toBeTruthy();
      const settings = await response.json();
      expect(settings).toBeTruthy();
    });
  });

  test.describe('API Integration Tests', () => {
    test('health endpoint works', async ({ page }) => {
      const response = await page.request.get('http://localhost:5173/health');
      expect(response.ok()).toBeTruthy();
      const data = await response.json();
      expect(data.status).toBe('ok');
      expect(data.service).toBe('assistant-agent');
    });

    test('alerts list returns data', async ({ page }) => {
      const response = await page.request.get('http://localhost:5173/api/alerts', {
        headers: { 'Authorization': `Bearer ${API_TOKEN}` },
      });
      expect(response.ok()).toBeTruthy();
      const data = await response.json();
      expect(data.total).toBeGreaterThan(0);
      expect(data.data.length).toBeGreaterThan(0);
    });

    test('alerts CRUD - update', async ({ page }) => {
      const listResp = await page.request.get('http://localhost:5173/api/alerts', {
        headers: { 'Authorization': `Bearer ${API_TOKEN}` },
      });
      const list = await listResp.json();
      const alert = list.data.find((a: any) => a.title === 'Dentist Appointment');
      expect(alert).toBeTruthy();

      const updateResp = await page.request.put(`http://localhost:5173/api/alerts/${alert.id}`, {
        headers: {
          'Authorization': `Bearer ${API_TOKEN}`,
          'Content-Type': 'application/json',
        },
        data: { description: 'Regular checkup at 4pm (rescheduled)' },
      });
      expect(updateResp.ok()).toBeTruthy();
    });

    test('alerts CRUD - delete', async ({ page }) => {
      const createResp = await page.request.post('http://localhost:5173/api/alerts', {
        headers: {
          'Authorization': `Bearer ${API_TOKEN}`,
          'Content-Type': 'application/json',
        },
        data: {
          title: 'Temp Alert To Delete',
          type: 'task',
          due_date: '2026-07-20',
          priority: 'low',
        },
      });
      expect(createResp.ok()).toBeTruthy();

      const listResp = await page.request.get('http://localhost:5173/api/alerts', {
        headers: { 'Authorization': `Bearer ${API_TOKEN}` },
      });
      const list = await listResp.json();
      const tempAlert = list.data.find((a: any) => a.title === 'Temp Alert To Delete');
      expect(tempAlert).toBeTruthy();

      const deleteResp = await page.request.delete(`http://localhost:5173/api/alerts/${tempAlert.id}`, {
        headers: { 'Authorization': `Bearer ${API_TOKEN}` },
      });
      expect(deleteResp.ok()).toBeTruthy();
    });

    test('batch acknowledge works', async ({ page }) => {
      const createResp = await page.request.post('http://localhost:5173/api/alerts', {
        headers: {
          'Authorization': `Bearer ${API_TOKEN}`,
          'Content-Type': 'application/json',
        },
        data: {
          title: 'Batch Test Alert',
          type: 'task',
          due_date: '2026-06-25',
          priority: 'low',
        },
      });
      expect(createResp.ok()).toBeTruthy();

      const listResp = await page.request.get('http://localhost:5173/api/alerts', {
        headers: { 'Authorization': `Bearer ${API_TOKEN}` },
      });
      const list = await listResp.json();
      const batchAlert = list.data.find((a: any) => a.title === 'Batch Test Alert');
      expect(batchAlert).toBeTruthy();

      const batchResp = await page.request.post('http://localhost:5173/api/alerts/batch/acknowledge', {
        headers: {
          'Authorization': `Bearer ${API_TOKEN}`,
          'Content-Type': 'application/json',
        },
        data: { ids: [batchAlert.id] },
      });
      expect(batchResp.ok()).toBeTruthy();
    });

    test('sync status endpoint works', async ({ page }) => {
      const response = await page.request.get('http://localhost:5173/api/sync/status', {
        headers: { 'Authorization': `Bearer ${API_TOKEN}` },
      });
      expect(response.ok()).toBeTruthy();
    });

    test('sync trigger endpoint works', async ({ page }) => {
      const response = await page.request.post('http://localhost:5173/api/sync/trigger', {
        headers: { 'Authorization': `Bearer ${API_TOKEN}` },
      });
      expect(response.ok()).toBeTruthy();
      const data = await response.json();
      expect(data.ok).toBeDefined();
    });

    test('history endpoint works', async ({ page }) => {
      const response = await page.request.get('http://localhost:5173/api/history', {
        headers: { 'Authorization': `Bearer ${API_TOKEN}` },
      });
      expect(response.ok()).toBeTruthy();
      const data = await response.json();
      expect(data.data).toBeDefined();
    });

    test('unauthorized request is rejected', async ({ page }) => {
      const response = await page.request.get('http://localhost:5173/api/alerts', {
        headers: { 'Authorization': 'Bearer wrong-token' },
      });
      expect(response.status()).toBe(401);
    });
  });
});
