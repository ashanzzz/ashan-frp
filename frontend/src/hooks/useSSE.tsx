import { useEffect, useState } from 'react';

export const useSSE = (url: string) => {
  const [data, setData] = useState<any>(null);

  useEffect(() => {
    const eventSource = new EventSource(url);
    eventSource.onmessage = (event) => {
      const parsed = JSON.parse(event.data);
      setData(parsed);
    };
    return () => eventSource.close();
  }, [url]);

  return data;
};
