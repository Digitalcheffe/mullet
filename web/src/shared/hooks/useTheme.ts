import { createContext, useContext } from 'react';
import { defaultTheme, type ThemeTokens } from '../themes/tokens';

export const ThemeContext = createContext<ThemeTokens>(defaultTheme);

export function useTheme(): ThemeTokens {
  return useContext(ThemeContext);
}
