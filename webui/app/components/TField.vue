<script setup lang="ts">
withDefaults(defineProps<{
  label: string;
  modelValue?: string | number;
  type?: string;
  placeholder?: string;
  textarea?: boolean;
  rows?: number;
  id?: string;
  readonly?: boolean;
}>(), {
  type: "text",
  rows: 3,
});

defineEmits<{ "update:modelValue": [v: string] }>();

const uid = useId();
</script>

<template>
  <div class="t-field">
    <label :for="id ?? uid">{{ label }}</label>
    <textarea
      v-if="textarea"
      :id="id ?? uid"
      :value="modelValue"
      :placeholder="placeholder"
      :rows="rows"
      :readonly="readonly"
      @input="$emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)"
    />
    <input
      v-else
      :id="id ?? uid"
      :type="type"
      :value="modelValue"
      :placeholder="placeholder"
      :readonly="readonly"
      @input="$emit('update:modelValue', ($event.target as HTMLInputElement).value)"
    />
  </div>
</template>
