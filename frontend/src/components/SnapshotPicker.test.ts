import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { main } from '../../wailsjs/go/models'
import SnapshotPicker from './SnapshotPicker.vue'

function snapshot(overrides: Partial<main.SnapshotDTO> = {}): main.SnapshotDTO {
  return {
    id: 1,
    seasonNum: 2,
    capturedAt: '2026-07-19T12:30:00Z',
    fileSizeBytes: 1024,
    fileExists: true,
    ...overrides,
  } as main.SnapshotDTO
}

function labelFor(overrides: Partial<main.SnapshotDTO>): string {
  const wrapper = mount(SnapshotPicker, {
    props: {
      snapshots: [snapshot(overrides)],
      loading: false,
      selectedId: null,
    },
  })
  return wrapper.get('.snapshot-season').text()
}

describe('SnapshotPicker', () => {
  it.each([
    [{ phase: 'preseason' }, 'Season 2 - Pre-season'],
    [
      { phase: 'regular_season', gameNumber: 4, opponentTeamName: 'Firebirds', isHome: true },
      'Season 2 - After Game 4 (vs Firebirds)',
    ],
    [
      { phase: 'regular_season', gameNumber: 5, opponentTeamName: 'Hares', isHome: false },
      'Season 2 - After Game 5 (@ Hares)',
    ],
    [{ phase: 'end_regular_season' }, 'Season 2 - End of regular season'],
    [{ phase: 'playoffs' }, 'Season 2 - Playoffs'],
    [
      { phase: 'playoffs', gameNumber: 2, opponentTeamName: 'Divine', isHome: true },
      'Season 2 - After playoff game 2 (vs Divine)',
    ],
    [
      { phase: 'playoffs', gameNumber: 1, opponentTeamName: 'Aquatics', isHome: false },
      'Season 2 - After playoff game 1 (@ Aquatics)',
    ],
    [{ phase: 'playoffs_eliminated' }, 'Season 2 - Playoffs (Team eliminated)'],
    [{ phase: 'end_season' }, 'Season 2 - End of season'],
    [{}, 'Season 2'],
  ])('renders phase label %#', (metadata, expected) => {
    expect(labelFor(metadata)).toBe(expected)
  })

  it('preserves the secondary capture details and missing-file presentation', () => {
    const wrapper = mount(SnapshotPicker, {
      props: {
        snapshots: [snapshot({ phase: 'preseason', fileExists: false })],
        loading: false,
        selectedId: null,
      },
    })

    expect(wrapper.get('.snapshot-season').text()).toBe('Season 2 - Pre-season')
    expect(wrapper.get('.snapshot-meta').text()).toContain('1.0 KB')
    expect(wrapper.get('.snapshot-badge-missing').text()).toBe('File missing')
    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
  })
})
