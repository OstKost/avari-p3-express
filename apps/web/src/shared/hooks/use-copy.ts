import { useState, useCallback } from 'react';
import { toast } from 'sonner';

export function useCopyToClipboard() {
  const [copiedText, setCopiedText] = useState<string | null>(null);

  const copy = useCallback(async (text: string, label: string = 'Ссылка') => {
    if (!navigator?.clipboard) {
      toast.error('Буфер обмена недоступен в этом браузере');
      return false;
    }

    try {
      await navigator.clipboard.writeText(text);
      setCopiedText(text);
      toast.success(`${label}: скопировано`);
      setTimeout(() => setCopiedText(null), 2000);
      return true;
    } catch {
      toast.error('Не удалось скопировать ссылку');
      return false;
    }
  }, []);

  return { copiedText, copy };
}
