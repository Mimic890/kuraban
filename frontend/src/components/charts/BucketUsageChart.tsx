import { useEffect, useState } from 'react';
import { Cell, Legend, Pie, PieChart, ResponsiveContainer, Tooltip } from 'recharts';
import type { BucketUsage } from '@/types';
import { formatBytes } from '@/lib/file-utils';
import { getUniqueThemeColors, getTextColor, getTooltipStyle } from '@/lib/chart-colors';

interface BucketUsageChartProps {
  data: BucketUsage[];
}

export function BucketUsageChart({ data }: BucketUsageChartProps) {
  const [colors, setColors] = useState<string[]>([]);
  const [textColor, setTextColorState] = useState('#e8eaed');

  useEffect(() => {
    const updateColors = () => {
      setColors(getUniqueThemeColors(data.length));
      setTextColorState(getTextColor());
    };

    updateColors();

    const observer = new MutationObserver(updateColors);
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });

    return () => observer.disconnect();
  }, [data.length]);

  const tooltipStyle = getTooltipStyle();

  const chartData = data.map((item) => ({
    name: item.bucketName,
    value: item.size,
    displaySize: formatBytes(item.size),
  }));

  return (
    <ResponsiveContainer width="100%" height={300}>
      <PieChart>
        <Pie
          data={chartData}
          cx="50%"
          cy="50%"
          labelLine={false}
          outerRadius={85}
          innerRadius={45}
          paddingAngle={3}
          dataKey="value"
        >
          {chartData.map((_, index) => (
            <Cell key={`cell-${index}`} fill={colors[index] || '#3f68c0'} stroke="var(--card)" strokeWidth={2} />
          ))}
        </Pie>
        <Tooltip
          formatter={(value) => formatBytes(value as number)}
          contentStyle={tooltipStyle as React.CSSProperties}
          labelStyle={{ color: 'var(--popover-foreground)' }}
          itemStyle={{ color: 'var(--popover-foreground)' }}
        />
        <Legend wrapperStyle={{ color: textColor }} />
      </PieChart>
    </ResponsiveContainer>
  );
}
