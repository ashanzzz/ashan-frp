import React from 'react';
import { Card, Row, Col, Switch, Badge, message } from 'antd';
import { CloudServerOutlined } from '@ant-design/icons';
import { useSSE } from '../hooks/useSSE';

export const TunnelsPage: React.FC = () => {
  const tunnels = useSSE('/api/sse/tunnels') || [];

  const handleToggle = async (name: string, active: boolean) => {
    const action = active ? 'start' : 'stop';
    try {
      await fetch(`/api/tunnels/${name}/${action}`, { method: 'POST' });
      message.success(`隧道【${name}】状态已成功重置`);
    } catch {
      message.error('通信熔断');
    }
  };

  return (
    <div style={{ background: '#0D0E12', minHeight: '100vh', padding: '24px', color: '#E3E4E8' }}>
      <Row g={24}>
        {tunnels.map((t: any) => (
          <Col span={8} key={t.id}>
            <Card style={{ background: '#16171F', border: '1px solid #232530', borderRadius: '12px' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span style={{ color: '#FFF', fontWeight: 600 }}><CloudServerOutlined /> {t.name}</span>
                <Badge status={t.status === 'running' ? 'processing' : 'default'} />
                <Switch checked={t.status === 'running'} onChange={(checked) => handleToggle(t.name, checked)} />
              </div>
              <div style={{ marginTop: '12px', background: '#0D0E12', padding: '8px', borderRadius: '6px' }}>
                <div style={{ fontSize: '12px', color: '#626875' }}>1Panel 联动状态: <span style={{ color: t.status_1panel === 'success' ? '#52c41a' : '#ff4d4f' }}>{t.status_1panel}</span></div>
                <div style={{ fontSize: '12px', color: '#626875' }}>chmlfrp 状态: <span style={{ color: t.status_chml === 'success' ? '#52c41a' : '#ff4d4f' }}>{t.status_chml}</span></div>
              </div>
            </Card>
          </Col>
        ))}
      </Row>
    </div>
  );
};
