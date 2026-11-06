import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen } from '@testing-library/react';
import { DialogContent, DialogTitle } from './dialog';
import { ConfirmDialog, GuardedDialog } from './confirm-dialog';

function Form({ dirty, onClose }: { dirty: boolean; onClose: () => void }) {
  const [, rerender] = useState(0);
  return (
    <GuardedDialog open onOpenChange={onClose} dirty={dirty}>
      <DialogContent>
        <DialogTitle>Form</DialogTitle>
        <input aria-label="first" />
        <input aria-label="second" onChange={() => rerender((n) => n + 1)} />
      </DialogContent>
    </GuardedDialog>
  );
}

const CLOSE_X = 'Close dialog';
const pressEscape = () => fireEvent.keyDown(document, { key: 'Escape' });
const question = () => screen.queryByText('Close this window?');

describe('GuardedDialog', () => {
  it('closes at once when nothing would be lost', () => {
    const onClose = vi.fn();
    render(<Form dirty={false} onClose={onClose} />);
    pressEscape();
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(question()).toBeNull();
  });

  it('asks first, and Esc on the question only dismisses the question', () => {
    const onClose = vi.fn();
    render(<Form dirty onClose={onClose} />);
    pressEscape();
    expect(question()).not.toBeNull();
    pressEscape();
    expect(question()).toBeNull();
    expect(onClose).not.toHaveBeenCalled();
  });

  it('closes after the user confirms', () => {
    const onClose = vi.fn();
    render(<Form dirty onClose={onClose} />);
    fireEvent.click(screen.getByLabelText(CLOSE_X));
    fireEvent.click(screen.getByRole('button', { name: 'Close' }));
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(question()).toBeNull();
  });

  it('keeps focus on the field being typed in when the parent rerenders', async () => {
    render(<Form dirty onClose={vi.fn()} />);
    await new Promise((resolve) => setTimeout(resolve));
    const second = screen.getByLabelText('second');
    second.focus();
    fireEvent.change(second, { target: { value: 'a' } });
    await new Promise((resolve) => setTimeout(resolve));
    expect(document.activeElement).toBe(second);
  });
});

describe('ConfirmDialog with a backup warning', () => {
  const button = (name: RegExp) => screen.getByRole('button', { name }) as HTMLButtonElement;
  const wait = (ms: number) => act(() => vi.advanceTimersByTime(ms));

  it('holds the button back for 5 seconds, and again on every opening', () => {
    vi.useFakeTimers();
    try {
      const onConfirm = vi.fn();
      const dialog = (open: boolean) => <ConfirmDialog open={open} onOpenChange={vi.fn()} title="Apply?" backupWarning onConfirm={onConfirm} />;
      const { rerender } = render(dialog(true));
      expect(screen.getByRole('alert').textContent).toContain('backup');
      expect(button(/Continue anyway \(5\)/).disabled).toBe(true);

      for (let i = 0; i < 4; i++) wait(1000);
      fireEvent.click(button(/Continue anyway \(1\)/));
      expect(onConfirm).not.toHaveBeenCalled();

      wait(1000);
      fireEvent.click(button(/^Continue anyway$/));
      expect(onConfirm).toHaveBeenCalledTimes(1);

      rerender(dialog(false));
      rerender(dialog(true));
      expect(button(/Continue anyway \(5\)/).disabled).toBe(true);
    } finally {
      vi.useRealTimers();
    }
  });
});

