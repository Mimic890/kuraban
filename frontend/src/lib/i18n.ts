import { useSettingsStore } from '@/store/settings-store';
import en from './locales/en.json';
import ru from './locales/ru.json';

const dictionaries: Record<string, Record<string, string>> = { en, ru };

type TranslationValues = Record<string, string | number>;

export function translate(key: string, values: TranslationValues = {}) {
  const language = useSettingsStore.getState().language;
  const template = (dictionaries[language] || dictionaries.en)[key] || dictionaries.en[key] || key;
  return template.replace(/\{\{(\w+)\}\}/g, (_, name: string) => String(values[name] ?? `{{${name}}}`));
}

export function useTranslation() {
  const language = useSettingsStore((state) => state.language);
  const dict = dictionaries[language] || dictionaries.en;
  const t = (key: string, values: TranslationValues = {}) =>
    (dict[key] || dictionaries.en[key] || key).replace(/\{\{(\w+)\}\}/g, (_, name: string) => String(values[name] ?? `{{${name}}}`));
  
  return { t, language };
}
