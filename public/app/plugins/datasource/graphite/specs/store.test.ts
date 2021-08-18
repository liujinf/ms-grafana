import { dispatch } from 'app/store/store';
import gfunc from '../gfunc';
import { TemplateSrvStub } from 'test/specs/helpers';
import { silenceConsoleOutput } from 'test/core/utils/silenceConsoleOutput';
import { actions } from '../state/actions';
import { getAltSegmentsSelectables, getTagsSelectables, getTagsAsSegmentsSelectables } from '../state/providers';
import { GraphiteSegment } from '../types';
import { createStore } from '../state/store';

jest.mock('app/core/utils/promiseToDigest', () => ({
  promiseToDigest: (scope: any) => {
    return (p: Promise<any>) => p;
  },
}));

jest.mock('app/store/store', () => ({
  dispatch: jest.fn(),
}));
const mockDispatch = dispatch as jest.Mock;

/**
 * Simulate switching to text editor, changing the query and switching back to visual editor
 */
async function changeTarget(ctx: any, target: string): Promise<void> {
  await ctx.dispatch(actions.toggleEditorMode());
  await ctx.dispatch(actions.updateQuery({ query: target }));
  await ctx.dispatch(actions.runQuery());
  await ctx.dispatch(actions.toggleEditorMode());
}

describe('Graphite actions', async () => {
  const ctx = {
    datasource: {
      metricFindQuery: jest.fn(() => Promise.resolve([])),
      getFuncDefs: jest.fn(() => Promise.resolve(gfunc.getFuncDefs('1.0'))),
      getFuncDef: gfunc.getFuncDef,
      waitForFuncDefsLoaded: jest.fn(() => Promise.resolve(null)),
      createFuncInstance: gfunc.createFuncInstance,
      getTagsAutoComplete: jest.fn().mockReturnValue(Promise.resolve([])),
    },
    target: { target: 'aliasByNode(scaleToSeconds(test.prod.*,1),2)' },
  } as any;

  beforeEach(async () => {
    jest.clearAllMocks();

    ctx.state = null;
    ctx.dispatch = createStore((state) => {
      ctx.state = state;
    });

    await ctx.dispatch(
      actions.init({
        datasource: ctx.datasource,
        target: ctx.target,
        refresh: jest.fn(),
        queries: [],
        //@ts-ignore
        templateSrv: new TemplateSrvStub(),
      })
    );
  });

  describe('init', () => {
    it('should validate metric key exists', () => {
      expect(ctx.datasource.metricFindQuery.mock.calls[0][0]).toBe('test.prod.*');
    });

    it('should not delete last segment if no metrics are found', () => {
      expect(ctx.state.segments[2].value).not.toBe('select metric');
      expect(ctx.state.segments[2].value).toBe('*');
    });

    it('should parse expression and build function model', () => {
      expect(ctx.state.queryModel.functions.length).toBe(2);
    });
  });

  describe('when toggling edit mode to raw and back again', () => {
    beforeEach(async () => {
      await ctx.dispatch(actions.toggleEditorMode());
      await ctx.dispatch(actions.toggleEditorMode());
    });

    it('should validate metric key exists', () => {
      const lastCallIndex = ctx.datasource.metricFindQuery.mock.calls.length - 1;
      expect(ctx.datasource.metricFindQuery.mock.calls[lastCallIndex][0]).toBe('test.prod.*');
    });

    it('should delete last segment if no metrics are found', () => {
      expect(ctx.state.segments[0].value).toBe('test');
      expect(ctx.state.segments[1].value).toBe('prod');
      expect(ctx.state.segments[2].value).toBe('select metric');
    });

    it('should parse expression and build function model', () => {
      expect(ctx.state.queryModel.functions.length).toBe(2);
    });
  });

  describe('when middle segment value of test.prod.* is changed', () => {
    beforeEach(async () => {
      const segment: GraphiteSegment = { type: 'metric', value: 'test', expandable: true };
      await ctx.dispatch(actions.segmentValueChanged({ segment: segment, index: 1 }));
    });

    it('should validate metric key exists', () => {
      const lastCallIndex = ctx.datasource.metricFindQuery.mock.calls.length - 1;
      expect(ctx.datasource.metricFindQuery.mock.calls[lastCallIndex][0]).toBe('test.test.*');
    });

    it('should delete last segment if no metrics are found', () => {
      expect(ctx.state.segments[0].value).toBe('test');
      expect(ctx.state.segments[1].value).toBe('test');
      expect(ctx.state.segments[2].value).toBe('select metric');
    });

    it('should parse expression and build function model', () => {
      expect(ctx.state.queryModel.functions.length).toBe(2);
    });
  });

  describe('when adding function', () => {
    beforeEach(async () => {
      ctx.state.datasource.metricFindQuery = () => Promise.resolve([{ expandable: false }]);
      await changeTarget(ctx, 'test.prod.*.count');
      await ctx.dispatch(actions.addFunction({ name: 'aliasByNode' }));
    });

    it('should add function with correct node number', () => {
      expect(ctx.state.queryModel.functions[0].params[0]).toBe(2);
    });

    it('should update target', () => {
      expect(ctx.state.target.target).toBe('aliasByNode(test.prod.*.count, 2)');
    });

    it('should call refresh', () => {
      expect(ctx.state.refresh).toHaveBeenCalled();
    });
  });

  describe('when adding function before any metric segment', () => {
    beforeEach(async () => {
      ctx.state.datasource.metricFindQuery = () => Promise.resolve([{ expandable: true }]);
      await changeTarget(ctx, '');
      await ctx.dispatch(actions.addFunction({ name: 'asPercent' }));
    });

    it('should add function and remove select metric link', () => {
      expect(ctx.state.segments.length).toBe(0);
    });
  });

  describe('when initializing a target with single param func using variable', () => {
    beforeEach(async () => {
      ctx.state.datasource.metricFindQuery = () => Promise.resolve([]);
      await changeTarget(ctx, 'movingAverage(prod.count, $var)');
    });

    it('should add 2 segments', () => {
      expect(ctx.state.segments.length).toBe(2);
    });

    it('should add function param', () => {
      expect(ctx.state.queryModel.functions[0].params.length).toBe(1);
    });
  });

  describe('when initializing target without metric expression and function with series-ref', () => {
    beforeEach(async () => {
      ctx.state.datasource.metricFindQuery = () => Promise.resolve([]);
      await changeTarget(ctx, 'asPercent(metric.node.count, #A)');
    });

    it('should add segments', () => {
      expect(ctx.state.segments.length).toBe(3);
    });

    it('should have correct func params', () => {
      expect(ctx.state.queryModel.functions[0].params.length).toBe(1);
    });
  });

  describe('when getting altSegments and metricFindQuery returns empty array', () => {
    beforeEach(async () => {
      ctx.state.datasource.metricFindQuery = () => Promise.resolve([]);
      await changeTarget(ctx, 'test.count');
      ctx.altSegments = await getAltSegmentsSelectables(ctx.state, 1, '');
    });

    it('should have no segments', () => {
      expect(ctx.altSegments.length).toBe(0);
    });
  });

  describe('when autocomplete for metric names is not available', () => {
    silenceConsoleOutput();
    beforeEach(() => {
      ctx.state.datasource.getTagsAutoComplete = jest.fn().mockReturnValue(Promise.resolve([]));
      ctx.state.datasource.metricFindQuery = jest.fn().mockReturnValue(
        new Promise(() => {
          throw new Error();
        })
      );
    });

    it('getAltSegmentsSelectables should handle autocomplete errors', async () => {
      await expect(async () => {
        await getAltSegmentsSelectables(ctx.state, 0, 'any');
        expect(mockDispatch).toBeCalledWith(
          expect.objectContaining({
            type: 'appNotifications/notifyApp',
          })
        );
      }).not.toThrow();
    });

    it('getAltSegmentsSelectables should display the error message only once', async () => {
      await getAltSegmentsSelectables(ctx.state, 0, 'any');
      expect(mockDispatch.mock.calls.length).toBe(1);

      await getAltSegmentsSelectables(ctx.state, 0, 'any');
      expect(mockDispatch.mock.calls.length).toBe(1);
    });
  });

  describe('when autocomplete for tags is not available', () => {
    silenceConsoleOutput();
    beforeEach(() => {
      ctx.datasource.metricFindQuery = jest.fn().mockReturnValue(Promise.resolve([]));
      ctx.datasource.getTagsAutoComplete = jest.fn().mockReturnValue(
        new Promise(() => {
          throw new Error();
        })
      );
    });

    it('getTagsSelectables should handle autocomplete errors', async () => {
      await expect(async () => {
        await getTagsSelectables(ctx.state, 0, 'any');
        expect(mockDispatch).toBeCalledWith(
          expect.objectContaining({
            type: 'appNotifications/notifyApp',
          })
        );
      }).not.toThrow();
    });

    it('getTagsSelectables should display the error message only once', async () => {
      await getTagsSelectables(ctx.state, 0, 'any');
      expect(mockDispatch.mock.calls.length).toBe(1);

      await getTagsSelectables(ctx.state, 0, 'any');
      expect(mockDispatch.mock.calls.length).toBe(1);
    });

    it('getTagsAsSegmentsSelectables should handle autocomplete errors', async () => {
      await expect(async () => {
        await getTagsAsSegmentsSelectables(ctx.state, 'any');
        expect(mockDispatch).toBeCalledWith(
          expect.objectContaining({
            type: 'appNotifications/notifyApp',
          })
        );
      }).not.toThrow();
    });

    it('getTagsAsSegmentsSelectables should display the error message only once', async () => {
      await getTagsAsSegmentsSelectables(ctx.state, 'any');
      expect(mockDispatch.mock.calls.length).toBe(1);

      await getTagsAsSegmentsSelectables(ctx.state, 'any');
      expect(mockDispatch.mock.calls.length).toBe(1);
    });
  });

  describe('targetChanged', () => {
    beforeEach(async () => {
      const newQuery = 'aliasByNode(scaleToSeconds(test.prod.*, 1), 2)';
      ctx.state.datasource.metricFindQuery = () => Promise.resolve([{ expandable: false }]);
      await changeTarget(ctx, newQuery);
    });

    it('should rebuild target after expression model', () => {
      expect(ctx.state.target.target).toBe('aliasByNode(scaleToSeconds(test.prod.*, 1), 2)');
    });

    it('should call refresh', () => {
      expect(ctx.state.refresh).toHaveBeenCalled();
    });
  });

  describe('when updating targets with nested query', () => {
    beforeEach(async () => {
      ctx.state.datasource.metricFindQuery = () => Promise.resolve([{ expandable: false }]);
      await changeTarget(ctx, 'scaleToSeconds(#A, 60)');
    });

    it('should add function params', () => {
      expect(ctx.state.queryModel.segments.length).toBe(1);
      expect(ctx.state.queryModel.segments[0].value).toBe('#A');

      expect(ctx.state.queryModel.functions[0].params.length).toBe(1);
      expect(ctx.state.queryModel.functions[0].params[0]).toBe(60);
    });

    it('target should remain the same', () => {
      expect(ctx.state.target.target).toBe('scaleToSeconds(#A, 60)');
    });

    it('targetFull should include nested queries', async () => {
      ctx.state.queries = [
        {
          target: 'nested.query.count',
          refId: 'A',
        },
      ];

      await changeTarget(ctx, ctx.target.target);

      expect(ctx.state.target.target).toBe('scaleToSeconds(#A, 60)');

      expect(ctx.state.target.targetFull).toBe('scaleToSeconds(nested.query.count, 60)');
    });
  });

  describe('when updating target used in other query', () => {
    beforeEach(async () => {
      ctx.datasource.metricFindQuery = () => Promise.resolve([{ expandable: false }]);
      ctx.state.target.refId = 'A';
      await changeTarget(ctx, 'metrics.foo.count');

      ctx.state.queries = [ctx.state.target, { target: 'sumSeries(#A)', refId: 'B' }];

      await changeTarget(ctx, 'metrics.bar.count');
    });

    it('targetFull of other query should update', () => {
      expect(ctx.state.queries[1].targetFull).toBe('sumSeries(metrics.bar.count)');
    });
  });

  describe('when adding seriesByTag function', () => {
    beforeEach(async () => {
      ctx.state.datasource.metricFindQuery = () => Promise.resolve([{ expandable: false }]);
      await changeTarget(ctx, '');
      await ctx.dispatch(actions.addFunction({ name: 'seriesByTag' }));
    });

    it('should update functions', () => {
      expect(ctx.state.queryModel.getSeriesByTagFuncIndex()).toBe(0);
    });

    it('should update seriesByTagUsed flag', () => {
      expect(ctx.state.queryModel.seriesByTagUsed).toBe(true);
    });

    it('should update target', () => {
      expect(ctx.state.target.target).toBe('seriesByTag()');
    });

    it('should call refresh', () => {
      expect(ctx.state.refresh).toHaveBeenCalled();
    });
  });

  describe('when parsing seriesByTag function', () => {
    beforeEach(async () => {
      ctx.state.datasource.metricFindQuery = () => Promise.resolve([{ expandable: false }]);
      await changeTarget(ctx, "seriesByTag('tag1=value1', 'tag2!=~value2')");
    });

    it('should add tags', () => {
      const expected = [
        { key: 'tag1', operator: '=', value: 'value1' },
        { key: 'tag2', operator: '!=~', value: 'value2' },
      ];
      expect(ctx.state.queryModel.tags).toEqual(expected);
    });
  });

  describe('when tag added', () => {
    beforeEach(async () => {
      ctx.state.datasource.metricFindQuery = () => Promise.resolve([{ expandable: false }]);
      await changeTarget(ctx, 'seriesByTag()');
      await ctx.dispatch(actions.addNewTag({ segment: { value: 'tag1' } }));
    });

    it('should update tags with default value', () => {
      const expected = [{ key: 'tag1', operator: '=', value: '' }];
      expect(ctx.state.queryModel.tags).toEqual(expected);
    });

    it('should update target', () => {
      const expected = "seriesByTag('tag1=')";
      expect(ctx.state.target.target).toEqual(expected);
    });
  });

  describe('when tag changed', () => {
    beforeEach(async () => {
      ctx.state.datasource.metricFindQuery = () => Promise.resolve([{ expandable: false }]);
      await changeTarget(ctx, "seriesByTag('tag1=value1', 'tag2!=~value2')");
      await ctx.dispatch(actions.tagChanged({ tag: { key: 'tag1', operator: '=', value: 'new_value' }, index: 0 }));
    });

    it('should update tags', () => {
      const expected = [
        { key: 'tag1', operator: '=', value: 'new_value' },
        { key: 'tag2', operator: '!=~', value: 'value2' },
      ];
      expect(ctx.state.queryModel.tags).toEqual(expected);
    });

    it('should update target', () => {
      const expected = "seriesByTag('tag1=new_value', 'tag2!=~value2')";
      expect(ctx.state.target.target).toEqual(expected);
    });
  });

  describe('when tag removed', () => {
    beforeEach(async () => {
      ctx.state.datasource.metricFindQuery = () => Promise.resolve([{ expandable: false }]);
      await changeTarget(ctx, "seriesByTag('tag1=value1', 'tag2!=~value2')");
      await ctx.dispatch(
        actions.tagChanged({ tag: { key: ctx.state.removeTagValue, operator: '=', value: '' }, index: 0 })
      );
    });

    it('should update tags', () => {
      const expected = [{ key: 'tag2', operator: '!=~', value: 'value2' }];
      expect(ctx.state.queryModel.tags).toEqual(expected);
    });

    it('should update target', () => {
      const expected = "seriesByTag('tag2!=~value2')";
      expect(ctx.state.target.target).toEqual(expected);
    });
  });
});
