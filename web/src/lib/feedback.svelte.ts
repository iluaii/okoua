// SPDX-License-Identifier: AGPL-3.0-or-later

// Lets any component open the feedback dialog (owned by the root layout),
// optionally with a category and subcategory already chosen.

type Open = (category?: string, subcategory?: string) => void;

export const feedbackDialog = $state<{ open: Open | null }>({ open: null });
